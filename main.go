package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to config.yml")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("FATAL: %v", err)
		os.Exit(1)
	}

	var notifier Notifier
	if cfg.Matrix.Encryption {
		bot, err := NewMatrixBot(&cfg.Matrix)
		if err != nil {
			log.Printf("FATAL: failed to initialize Matrix bot: %v", err)
			os.Exit(1)
		}
		defer bot.Close()
		notifier = bot
	} else {
		notifier = &LegacyNotifier{cfg: cfg}
	}

	srv := RunServer(cfg, notifier)

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("INFO: Received %s, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("WARNING: shutdown timed out: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("FATAL: %v", err)
		os.Exit(1)
	}

	log.Printf("INFO: Pentarou stopped")
}
