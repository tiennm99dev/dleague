// Package main is the dleague server entry point.
// It connects to MongoDB, ensures indexes, and serves HTTP + WebSocket traffic.
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

	"github.com/tiennm99/dleague/server/internal/config"
	srvhttp "github.com/tiennm99/dleague/server/internal/http"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 15-second budget for Connect + Ping + EnsureIndexes.
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

	// Construct repos. They are not used by hub yet; later phases wire them in.
	_ = store.NewUserRepo(db)
	_ = store.NewGameRepo(db)
	_ = store.NewMatchRepo(db)
	_ = store.NewAttemptRepo(db)
	_ = store.NewDailyPuzzleRepo(db)
	_ = store.NewLeaderboardRepo(db)

	// In production, an empty origin allowlist means the WS endpoint accepts
	// any origin — a cross-site WebSocket hijacking risk. Fail fast.
	if cfg.IsProduction() && len(cfg.AllowedOrigins) == 0 {
		log.Fatalf("DLEAGUE_WS_ORIGINS must be non-empty in production")
	}

	hub := ws.NewHub()
	hub.MaxConns = cfg.MaxConns
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
