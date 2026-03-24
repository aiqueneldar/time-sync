// TimeSync backend server entry point.
//
// This binary:
//  1. Loads configuration from environment variables.
//  2. Registers all time-reporting system adapters.
//  3. Starts an HTTP (or HTTPS) server.
//
// TLS is enabled when TLS_ENABLED=true and TLS_CERT_FILE / TLS_KEY_FILE are set.
// For local development, run without TLS and use a reverse-proxy (nginx, Caddy)
// to handle certificates in production.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/adapters/fieldglass"
	"github.com/aiqueneldar/time-sync/backend/internal/adapters/maconomy"
	"github.com/aiqueneldar/time-sync/backend/internal/api"
	"github.com/aiqueneldar/time-sync/backend/internal/config"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
	synce "github.com/aiqueneldar/time-sync/backend/internal/sync"
)

func main() {
	// ── Configuration ─────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Adapter registry ──────────────────────────────────────────────────
	// To add a new time-reporting system:
	//  1. Implement the adapters.Adapter interface in a new sub-package.
	//  2. Add one line here: registry.Register(mysystem.New(...))
	registry := adapters.NewRegistry()

	registry.Register(maconomy.New(cfg.MaconomyBaseURL, cfg.MaconomyCompany))
	registry.Register(fieldglass.New())

	// ── Infrastructure ────────────────────────────────────────────────────
	sessions := session.NewStore()
	engine := synce.New(registry, sessions)

	// ── HTTP router ───────────────────────────────────────────────────────
	handler := api.NewRouter(registry, sessions, engine, cfg.AllowedOrigins)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,

		// Sensible timeouts to prevent Slowloris-style attacks (OWASP A05).
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second, // longer for SSE connections
		IdleTimeout:       120 * time.Second,
	}

	// ── Start ─────────────────────────────────────────────────────────────
	go func() {
		if cfg.TLSEnabled {
			if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
				log.Fatal("TLS_ENABLED=true but TLS_CERT_FILE or TLS_KEY_FILE not set")
			}
			log.Printf("TimeSync backend starting on https://0.0.0.0:%s (TLS)", cfg.Port)
			if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error: %v", err)
			}
		} else {
			log.Printf("TimeSync backend starting on http://0.0.0.0:%s (no TLS – use a reverse proxy in production)", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error: %v", err)
			}
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Forced shutdown: %v", err)
	}
	log.Println("Server stopped.")
}
