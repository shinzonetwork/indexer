package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shinzo/version1/config"
	"shinzo/version1/pkg/indexer"
)

func main() {
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create indexer
	idx, err := indexer.NewIndexer(cfg)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping indexer...")
		cancel()
	}()

	log.Printf("Starting indexer with DefraDB at %s:%d", cfg.DefraDB.Host, cfg.DefraDB.Port)

	// Start indexer
	if err := idx.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Indexer failed: %v", err)
	}
}
