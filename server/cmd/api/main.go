// Package main is the dleague server entry point.
// It connects to MongoDB, initialises Firebase Auth, and serves HTTP + WebSocket traffic.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/config"
	"github.com/tiennm99/dleague/server/internal/game/wordle"
	srvhttp "github.com/tiennm99/dleague/server/internal/http"
	"github.com/tiennm99/dleague/server/internal/scheduler"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/ws"
)

// roomTimeoutInterval is how often the rooms ticker checks match deadlines.
const roomTimeoutInterval = time.Second

// hasAtlasSRV reports whether the URI uses the Atlas SRV scheme.
func hasAtlasSRV(uri string) bool {
	return len(uri) > 14 && uri[:14] == "mongodb+srv://"
}

// decodeServiceAccount decodes FIREBASE_SERVICE_ACCOUNT_B64 (if set) to a temp
// file and sets GOOGLE_APPLICATION_CREDENTIALS so the Firebase Admin SDK picks
// it up via Application Default Credentials. No secret value is logged.
func decodeServiceAccount() {
	b64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_B64")
	if b64 == "" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		log.Fatalf("auth: base64 decode FIREBASE_SERVICE_ACCOUNT_B64: %v", err)
	}
	path := "/tmp/dleague-sa.json"
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Fatalf("auth: write service account to %s: %v", path, err)
	}
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path); err != nil {
		log.Fatalf("auth: setenv GOOGLE_APPLICATION_CREDENTIALS: %v", err)
	}
	log.Printf("auth: using decoded service-account from env")
}

func main() {
	decodeServiceAccount()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Production safety checks.
	if cfg.IsProduction() && !hasAtlasSRV(cfg.MongoURI) {
		log.Printf("WARNING: MONGO_URI does not use mongodb+srv:// — plain mongodb:// is not recommended for Atlas")
	}

	// 15-second budget for Connect + Ping + EnsureIndexes + Firebase init.
	bootCtx, cancelBoot := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelBoot()

	client, err := store.Connect(bootCtx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("store: connect: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("store: disconnect: %v", err)
		}
	}()

	if err := client.Ping(bootCtx); err != nil {
		log.Fatalf("store: ping: %v", err)
	}

	db := client.Database()

	if err := store.EnsureIndexes(bootCtx, db); err != nil {
		log.Fatalf("store: ensure indexes: %v", err)
	}

	userRepo := store.NewUserRepo(db)
	_ = store.NewGameRepo(db)
	matchRepo := store.NewMatchRepo(db)
	attemptRepo := store.NewAttemptRepo(db)
	dailyRepo := store.NewDailyPuzzleRepo(db)
	wordlistRepo := store.NewWordlistRepo(db)
	leaderboardRepo := store.NewLeaderboardRepo(db)

	// Load word lists (Mongo first, embedded fallback if collection is empty).
	answers, err := wordle.LoadAnswers(bootCtx, wordlistRepo)
	if err != nil {
		log.Printf("wordle: load answers: %v (using embedded fallback)", err)
		answers = wordle.EmbeddedAnswers()
	}
	dictionary, err := wordle.LoadDictionary(bootCtx, wordlistRepo)
	if err != nil {
		log.Printf("wordle: load dictionary: %v (using embedded fallback)", err)
		dictionary = wordle.EmbeddedDictionary()
	}

	// Best-effort: seed today's daily puzzle at startup.
	if _, err := wordle.EnsureToday(bootCtx, dailyRepo, answers, time.Now()); err != nil {
		log.Printf("wordle: EnsureToday at boot: %v", err)
	}

	// Initialise Firebase Auth verifier. Boot fails fast on misconfiguration.
	if cfg.FirebaseEmulatorHost != "" {
		log.Printf("firebase: using emulator at %s", cfg.FirebaseEmulatorHost)
	}
	verifier, err := auth.New(bootCtx, cfg.FirebaseProjectID, cfg.FirebaseCredsPath)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	// In production, an empty origin allowlist means the WS endpoint accepts
	// any origin — a cross-site WebSocket hijacking risk. Fail fast.
	if cfg.IsProduction() && len(cfg.AllowedOrigins) == 0 {
		log.Fatalf("DLEAGUE_WS_ORIGINS must be non-empty in production")
	}
	// Warn when a wildcard pattern is present in production — still operational
	// but likely misconfigured.
	if cfg.IsProduction() {
		for _, origin := range cfg.AllowedOrigins {
			if strings.Contains(origin, "*") {
				log.Printf("WARN: production AllowedOrigins contains wildcard: %q", origin)
			}
		}
	}

	// Phase 09: in-memory sync PvP state — created once, shared via GameDeps.
	syncQueue := ws.NewQueue()
	syncRooms := ws.NewRoomsRegistry()
	graceTimers := ws.NewGraceTimers()

	// Per-UID rate limiter: 20 msg/sec, burst 40, evict idle after 1h.
	uidLim := ws.NewUIDLimiter(20, 40, time.Hour)

	hub := ws.NewHub(verifier, userRepo)
	hub.MaxConns = cfg.MaxConns
	hub.GameDeps = &ws.GameDeps{
		DailyRepo:       dailyRepo,
		Dictionary:      dictionary,
		Answers:         answers,
		MatchRepo:       matchRepo,
		AttemptRepo:     attemptRepo,
		LeaderboardRepo: leaderboardRepo,
		UserRepo:        userRepo,
		MongoClient:     client.Inner(),
		Queue:           syncQueue,
		Rooms:           syncRooms,
		GraceTimers:     graceTimers,
		UIDLimiter:      uidLim,
	}
	wsOpts := ws.UpgradeOptions{AllowedOrigins: cfg.AllowedOrigins}

	rOpts := srvhttp.RouterOptions{TrustedProxies: cfg.TrustedProxies}
	r, err := srvhttp.NewRouter(cfg.WebRoot, hub, wsOpts, client, rOpts)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Evict idle UID buckets every 5 minutes.
	go uidLim.RunEvictor(ctx, 5*time.Minute)

	// Background scheduler: refresh leaderboards every 5 min, sweep expired
	// matches every 15 min. Tied to signal context — shuts down on SIGTERM.
	go scheduler.Run(ctx, scheduler.Config{}, scheduler.Repos{
		Leaderboard: leaderboardRepo,
		Match:       matchRepo,
	})

	// Phase 09: rooms timeout ticker — checks every second for expired matches.
	// Also piggybacks queue TTL eviction every 5 ticks. Phase 09 M6 fix.
	go func() {
		ticker := time.NewTicker(roomTimeoutInterval)
		defer ticker.Stop()
		var tick int
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tick++
				now := time.Now()
				for _, room := range syncRooms.All() {
					if !room.Deadline.IsZero() && now.After(room.Deadline) {
						room.HandleTimeout(context.Background(), hub.GameDeps)
					}
				}
				// Evict stale queue entries every 5 seconds.
				if tick%5 == 0 {
					syncQueue.EvictExpired(func(c *ws.Conn) {
						c.EnqueueError("queue_timeout")
					})
				}
			}
		}
	}()

	go func() {
		log.Printf("dleague server listening on %s (web=%s)", cfg.Addr, cfg.WebRoot)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")

	// Signal active WS clients before closing the HTTP server so they receive
	// a clean error frame rather than a bare TCP close.
	hub.CloseAll("server shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
