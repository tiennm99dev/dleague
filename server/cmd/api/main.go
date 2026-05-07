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
	srvhttp "github.com/tiennm99/dleague/server/internal/http"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/mongodb"
	"github.com/tiennm99/dleague/server/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	bootCtx, cancelBoot := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelBoot()

	mongoStore, err := mongodb.New(bootCtx, mongodb.Config{
		URI:      cfg.MongoURI,
		Database: cfg.MongoDB,
	})
	if err != nil {
		log.Fatalf("mongodb: %v", err)
	}
	defer func() {
		if err := mongoStore.Close(); err != nil {
			log.Printf("store close: %v", err)
		}
	}()

	var st store.Store = mongoStore

	verifier, err := auth.NewFirebase(bootCtx, cfg.FirebaseCredentialsJSON, cfg.FirebaseProjectID)
	if err != nil {
		log.Fatalf("firebase auth: %v", err)
	}

	hub := ws.NewHub()
	wsOpts := ws.UpgradeOptions{
		AllowedOrigins: cfg.AllowedOrigins,
		Verifier:       auth.NewGate(verifier, st),
	}

	r, err := srvhttp.NewRouter(cfg.WebRoot, hub, wsOpts, st, verifier)
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
