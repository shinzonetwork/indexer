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
	mode := flag.String("mode", "realtime", "Indexing mode: 'realtime' for real-time indexing, 'catchup' for catch-up indexing")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate and parse indexing mode
	_, err = parseIndexingMode(*mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Usage: %s -mode=[realtime|catchup]\n", os.Args[0])
		os.Exit(1)
	}

	// Start indexing with configuration
	modeEnum, _ := parseIndexingMode(*mode) // Already validated above
	if err := indexer.StartIndexingWithModeAndConfig("", cfg.DefraDB.Url, modeEnum, cfg); err != nil {
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

func findConfigFile() string {
	possiblePaths := []string{
		"./config.yaml",     // From project root
		"../config.yaml",    // From bin/ directory
		"../../config.yaml", // From pkg/host/ directory - test context
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "config.yaml"
}
