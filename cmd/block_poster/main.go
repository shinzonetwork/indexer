package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/indexer"
)

func main() {
	configPath := flag.String("config", "config/test.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create and start indexer
	chainIndexer := indexer.CreateIndexer(cfg)
	if err := chainIndexer.StartIndexing(false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start indexing: %v\n", err)
		os.Exit(1)
	}
}
