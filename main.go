package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/server"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// Injected during build via -ldflags
var (
	Version   = "v2.22.0"
	Commit    = "dev"
	BuildTime = "local"
)

//go:embed all:static
var staticFiles embed.FS

func main() {
	defaultAddr := envOr("BOTHUB_ADDR", ":8080")
	defaultDB := envOr("BOTHUB_DB", "./data/bothub.db")

	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	dbPath := flag.String("db", defaultDB, "SQLite database file path")
	showVersion := flag.Bool("v", false, "print version and exit")
	flag.BoolVar(showVersion, "version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("BotHub %s (commit: %s, built at: %s)\n", Version, Commit, BuildTime)
		os.Exit(0)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("[bothub] failed to open sqlite store (%s): %v", *dbPath, err)
	}
	defer st.Close()

	pushClient := notifier.NewClient(st)
	registry := modules.NewDefaultRegistry(st, pushClient)

	sched := scheduler.NewWithRegistry(st, registry)
	sched.SetHub(pushClient.Hub())
	if err := sched.Start(); err != nil {
		log.Fatalf("[bothub] failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	srvHandler, err := server.New(st, sched, pushClient, staticFiles, Version, Commit, BuildTime)
	if err != nil {
		log.Fatalf("[bothub] failed to initialize http server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srvHandler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0, // Disabled at server level to support long-lived SSE streams (/api/v2/events, /api/agent/chat)
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("[bothub] Vesper BotHub %s (%s) listening on %s (db=%s)", Version, Commit, *addr, *dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[bothub] http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[bothub] shutting down gracefully...")
	srvHandler.ClosePushStream()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
