package main

import (
	"context"
	"log"
	"path"
	"runtime"
	"shinzo/version1/config"
	"shinzo/version1/pkg/defra"
	"shinzo/version1/pkg/logger"

	"github.com/lens-vm/lens/host-go/config/model"
	"github.com/sourcenetwork/immutable"
)

// NewView{
//     decodedTopics
//     decodedData
//     decodedEventName
// }

func main() {
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger.Init(cfg.Logger.Development)
	sugar := logger.Sugar

	// queries
	// types

	// lenses parameters (abi, address, ...)

	viewHandler := defra.NewViewHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)
	// create view for test purposes, in real prod enviroment we get the data from either user input or cosmos blocks parsing
	view := viewHandler.CreateView(
		`			
			Log {
				address
				data
				topics
			}
		`,
		`
			type FilteredSomeLog12345 @materialized(if: false) {
				address: String,
				data: String,
				topics: String,
			}
		`,
		immutable.Some(model.Lens{
			Lenses: []model.LensModule{
				{
					Path: "file:///Users/daniel/Desktop/code/lenses/rust_wasm32_filter_contract_addresses/target/wasm32-unknown-unknown/debug/rust_wasm32_filter_contract_addresses.wasm",
					Arguments: map[string]any{
						"src":   "address",
						"value": "0x271590d858ef858ba411372d221035bf67014f41",
					},
				},
			},
		}),
	)

	schemaAndDesc, err := viewHandler.AddView(context.Background(), view, sugar)
	if err != nil {
		sugar.Error(err)
		return
	}
	sugar.Info(schemaAndDesc)
}

func getPathRelativeToProjectRoot(relativePath string) string {
	_, filename, _, _ := runtime.Caller(0)
	root := path.Dir(path.Dir(path.Dir(filename)))
	return "file://" + path.Join(root, relativePath)
}
