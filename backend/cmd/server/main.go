// TimeSync backend server entry point.
//
// This backend can either be run as a HTTP only backend for dev, or behind a reverse proxy that terminiate TLS.
// Or this server can be run with TLS directly. To do so make sure to configure TLS_CERT and TLS_KEY env variables

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/adapters/maconomy"
	"github.com/aiqueneldar/time-sync/backend/internal/api/handlers"
	"github.com/aiqueneldar/time-sync/backend/internal/api/middleware"
	"github.com/aiqueneldar/time-sync/backend/internal/config"
	"github.com/elliotxx/healthcheck"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// ── Adapter registry ──────────────────────────────────────────────────
	// To add a new time-reporting system:
	//  1. Implement the adapters.Adapter interface in a new sub-package.
	//  2. Add one line here: registry.Register(mysystem.New(...))
	registry := adapters.NewRegistry()

	registry.Register(maconomy.New(cfg))

	// ---- Setup middleware and routing for main http-server
	router := gin.Default()

	// Attach middleware for all endpoints
	router.Use(middleware.SecurityHeaders, middleware.CORS(cfg))

	// Attach Healtcheck endpoints
	healthcheck.Register(&router.RouterGroup)

	// Start adding in routes and group them to be able to attach middleware to all underlying routes for a group
	v1 := router.Group("/api/v1")

	systemsH := handlers.NewSystemsHandler(registry)
	authH := handlers.NewAuthHandler(registry)
	timecodesH := handlers.NewTimecodesHandler(registry)
	//syncH := handlers.NewSyncHandler(sessions, engine)

	//--- Register endpoints and handlers
	v1.GET("/systems", systemsH.Handler)
	v1.POST("/auth", authH.HandleAuth)
	v1.GET("/timecodes", timecodesH.GetTimecodes)

	//--- Internal healthcheck handler when running in docker
	if len(os.Args) == 2 && os.Args[1] == "-health" {
		ok, err := HealthCheck()
		if err != nil {
			log.Fatal(err) // Exit with healthcheck error
		}
		if ok {
			os.Exit(0)
		} else {
			os.Exit(2) // Exit with healthcheck working, but not ok
		}
	}

	// ── Start up Gin server
	srv := http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port), // Listen on 0.0.0.0:<PORT>
		Handler: router,

		// Sensible timeouts to prevent Slowloris-style attacks (OWASP A05).
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Starting up Time-Sync API server on 0.0.0.0:%s...\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Time-Sync API Listen error: %s", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)                      // make a listen channel to use for shutdown command
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // Register it with syscalls that will shut os down clean
	<-quit                                               // Listen indefinetly until such a signal is sent by the OS

	log.Println("Shutting Time-Sync API down gracefully…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Forced shutdown: %v", err)
	}
	log.Println("Time-Sync API Server stopped.")
}
