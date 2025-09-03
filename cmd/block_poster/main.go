package main

import (
	"flag"
	"fmt"

	"github.com/shinzonetwork/indexer/pkg/indexer"
)

func main() {
	defraStorePath := flag.String("defra-store-path", "", "Path to Defra store directory. If empty, assumes Defra is already running. Example: -defra-store-path=./.defra")
	defraUrl := flag.String("defra-url", "http://localhost:9181", "The URL your defra instance is running on. If you are not currently running a defra instance, please omit this flag.")
	mode := flag.String("mode", "realtime", "Indexing mode: 'realtime' for real-time indexing, 'catchup' for catch-up indexing")
	flag.Parse()

	var indexingMode indexer.IndexingMode
	switch *mode {
	case "catchup":
		indexingMode = indexer.ModeCatchUp
	case "realtime":
		indexingMode = indexer.ModeRealTime
	default:
		panic(fmt.Errorf("Invalid mode: %s. Use 'realtime' or 'catchup'", *mode))
	}

	err := indexer.StartIndexingWithMode(*defraStorePath, *defraUrl, indexingMode)
	if err != nil {
		panic(fmt.Errorf("Failed to start indexing: %v", err))
	}
}
