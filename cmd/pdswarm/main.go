package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/podiax/pdswarm"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/audit"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)
func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v", err)
	}

	if cfg.ShowVersion {
		fmt.Printf("PDSwarm v%s\n", config.Version)
		os.Exit(0)
	}

	// 2. Initialize security audit logger for fail2ban / jails
	auditLogger, err := audit.New(cfg.AuthLogPath, cfg.LogLevel, cfg.Port)
	if err != nil {
		log.Printf("[WARN] Error initializing security audit log at %s: %v", cfg.AuthLogPath, err)
	} else {
		defer auditLogger.Close()
		audit.SetDefault(auditLogger)
		if cfg.LogLevel != "off" {
			log.Printf("[INFO] Security audit logging enabled at %s (level: %s)", cfg.AuthLogPath, cfg.LogLevel)
		}
	}

	log.Printf("[INFO] Starting PDSwarm v%s...", config.Version)

	// 2. Initialize bbolt store
	dbPath := filepath.Join(cfg.DataDir, "pdswarm.db")
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database at %s: %v", dbPath, err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("[WARN] Error closing store: %v", err)
		}
	}()
	log.Printf("[INFO] Database initialized at %s", dbPath)

	// 3. Generate random JWT secret if none configured and none in database
	if cfg.JWTSecret == "" {
		storedSecret, err := st.GetSetting("jwt_secret")
		if err == nil && storedSecret != "" {
			cfg.JWTSecret = storedSecret
		} else {
			secretBytes := make([]byte, 32)
			if _, err := rand.Read(secretBytes); err != nil {
				log.Fatalf("[FATAL] Failed to generate random JWT secret: %v", err)
			}
			cfg.JWTSecret = hex.EncodeToString(secretBytes)
			if err := st.SaveSetting("jwt_secret", cfg.JWTSecret); err != nil {
				log.Printf("[WARN] Failed to persist generated JWT secret to database: %v", err)
			} else {
				log.Println("[INFO] Generated and persisted new random JWT secret")
			}
		}
	}

	// 4. Initialize Docker client with ping check
	ds, err := docker.NewService(cfg.SocketPath)
	if err != nil {
		log.Printf("[WARN] Failed to create Docker service: %v", err)
	} else {
		defer ds.Close()
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := ds.Ping(pingCtx); err != nil {
			log.Printf("[WARN] Docker daemon ping check failed at %s: %v", cfg.SocketPath, err)
		} else {
			log.Printf("[INFO] Docker daemon successfully connected at %s", cfg.SocketPath)
		}
		pingCancel()
	}

	// 5. Assemble chi router with static file server attached
	router := api.SetupRouter(cfg, st, ds, pdswarm.WebFS())

	// 6. Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("[INFO] PDSwarm server listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// 7. Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[INFO] Shutting down PDSwarm server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown error: %v", err)
	}

	log.Println("[INFO] PDSwarm stopped cleanly")
}
