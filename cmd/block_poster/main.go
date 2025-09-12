package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/indexer"
)

func main() {
	defraStarted := flag.Bool("defra-started", false, "Pass true if you are already using a defra instance you'd like to connect to. Otherwise, this flag can be omitted altogether - this app will start a defra instance for you")
	flag.Parse()

	configPath := findConfigFile()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		panic(fmt.Errorf("Unable to load config: %v", err))
	}

	err = indexer.StartIndexing(*defraStarted, cfg)
	if err != nil {
		panic(fmt.Errorf("Failed to start indexing: %v", err))
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
