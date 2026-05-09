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

	bootCtx, cancelBoot := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelBoot()

	st, err := store.New(bootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("store close: %v", err)
		}
	}()

	if err := store.Migrate(bootCtx, st.DB()); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied")

	// In production, an empty origin allowlist means the WS endpoint accepts
	// any origin — a cross-site WebSocket hijacking risk. Fail fast.
	if cfg.IsProduction() && len(cfg.AllowedOrigins) == 0 {
		log.Fatalf("DLEAGUE_WS_ORIGINS must be non-empty in production")
	}

	hub := ws.NewHub()
	hub.MaxConns = cfg.MaxConns
	wsOpts := ws.UpgradeOptions{AllowedOrigins: cfg.AllowedOrigins}

	rOpts := srvhttp.RouterOptions{TrustedProxies: cfg.TrustedProxies}
	r, err := srvhttp.NewRouter(cfg.WebRoot, hub, wsOpts, st, rOpts)
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
