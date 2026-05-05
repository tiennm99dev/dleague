package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	srvhttp "github.com/tiennm99/dleague/server/internal/http"
	"github.com/tiennm99/dleague/server/internal/ws"
)

func main() {
	addr := envOr("DLEAGUE_ADDR", ":8080")
	webRoot := envOr("DLEAGUE_WEB", "./web")

	var opts ws.UpgradeOptions
	if origins := os.Getenv("DLEAGUE_WS_ORIGINS"); origins != "" {
		opts.AllowedOrigins = splitCSV(origins)
	}

	hub := ws.NewHub()
	r, err := srvhttp.NewRouter(webRoot, hub, opts)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("dleague server listening on %s (web=%s)", addr, webRoot)
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
