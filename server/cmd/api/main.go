// Package main is the dleague server entry point.
// It connects to MongoDB, initialises Firebase Auth, and serves HTTP + WebSocket traffic.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
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

	// Background scheduler: refresh leaderboards every 5 min, sweep expired
	// matches every 15 min. Tied to signal context — shuts down on SIGTERM.
	go scheduler.Run(ctx, scheduler.Config{}, scheduler.Repos{
		Leaderboard: leaderboardRepo,
		Match:       matchRepo,
	})

	go func() {
		log.Printf("dleague server listening on %s (web=%s)", cfg.Addr, cfg.WebRoot)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
