package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shinzonetwork/indexer/pkg/indexer"
)

func main() {
	defraStorePath := flag.String("defra-store-path", "", "Path to DefraDB store directory. If empty, assumes DefraDB is already running.")
	defraUrl := flag.String("defra-url", "http://localhost:9181", "URL of the DefraDB instance.")
	mode := flag.String("mode", "realtime", "Indexing mode: 'realtime' for real-time indexing, 'catchup' for catch-up indexing")
	flag.Parse()

	// Validate and parse indexing mode
	_, err := parseIndexingMode(*mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Usage: %s -mode=[realtime|catchup]\n", os.Args[0])
		os.Exit(1)
	}

	// Start indexing with proper error handling
	if err := indexer.StartIndexingWithMode(*defraStorePath, *defraUrl, "realtime"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start indexing: %v\n", err)
		os.Exit(1)
	}
}

// parseIndexingMode validates and converts mode string to IndexingMode
func parseIndexingMode(mode string) (indexer.IndexingMode, error) {
	switch mode {
	case "catchup":
		return indexer.ModeCatchUp, nil
	case "realtime", "":
		return indexer.ModeRealTime, nil
	default:
		return "", fmt.Errorf("invalid mode: %s. Use 'realtime' or 'catchup'", mode)
	}
}
