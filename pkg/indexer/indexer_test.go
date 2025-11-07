package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	appdefra "github.com/shinzonetwork/app-sdk/pkg/defra"
	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestIndexing_StartDefraFirst(t *testing.T) {
	// Skip this test if we don't have a real Geth connection available
	// This test requires actual blockchain connectivity
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test - SKIP_INTEGRATION_TESTS is set")
	}

	logger.InitConsoleOnly(true)

	ctx := context.Background()
	
	// Use app-sdk to start defra instance for testing
	appConfig := appdefra.DefaultConfig
	schemaApplier := &appdefra.SchemaApplierFromFile{DefaultPath: "schema/schema.graphql"}
	indexerDefra, err := appdefra.StartDefraInstanceWithTestConfig(t, appConfig, schemaApplier, "Block", "Transaction", "AccessListEntry", "Log")
	require.NoError(t, err)
	defer indexerDefra.Close(ctx)

	port := defra.GetPort(indexerDefra)
	require.NotEqual(t, -1, port, "Unable to retrieve indexer's defra port")

	// Get the actual API URL from the defra node
	defraUrl := indexerDefra.APIURL
	
	_, err = queryBlockNumberFromUrl(ctx, defraUrl)
	require.Error(t, err)

	// Create test config by copying DefaultConfig and updating the URL
	// Use the actual defra URL (can be LAN IP or localhost)
	testCfg := &config.Config{}
	*testCfg = *DefaultConfig // Copy the config
	testCfg.ShinzoAppConfig.DefraDB.Url = defraUrl

	i := CreateIndexer(testCfg)
	go func() {
		err := i.StartIndexing(true)
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting defra dependency: %v", err))
		}
	}()

	for !i.IsStarted() || !i.HasIndexedAtLeastOneBlock() {
		time.Sleep(100 * time.Millisecond)
	}

	blockNumber, err := queryBlockNumberFromUrl(ctx, defraUrl)
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}

func queryBlockNumberFromUrl(ctx context.Context, defraUrl string) (int, error) {
	handler, err := defra.NewBlockHandler(defraUrl)
	if err != nil {
		return 0, fmt.Errorf("Error building block handler: %v", err)
	}
	query := `query GetHighestBlockNumber {
  Block(order: {number: DESC}, limit: 1) {
    number
  }
}`
	request := types.Request{Query: query, Type: "POST"}
	result, err := handler.SendToGraphql(ctx, request)
	if err != nil {
		return 0, fmt.Errorf("Error sending graphql query %s : %v", query, err)
	}

	var rawResponse map[string]interface{}
	if err := json.Unmarshal([]byte(result), &rawResponse); err != nil {
		return 0, fmt.Errorf("Error unmarshalling reponse: %v", err)
	}
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("Data field not found in response: %s", result)
	}
	blockBlob, ok := data["Block"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("Block field not found in response: %s", result)
	}
	if len(blockBlob) == 0 {
		return 0, fmt.Errorf("No blocks found in response: %s", result)
	}
	blockDataBlob, ok := blockBlob[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("Block field not found in response: %s", result)
	}
	blockNumberObject, ok := blockDataBlob["number"]
	if !ok {
		return 0, fmt.Errorf("Block number field not found in response: %s", result)
	}
	blockNumber, ok := blockNumberObject.(float64)
	if !ok {
		return 0, fmt.Errorf("Block number field not a number in response: %s", string(result))
	}
	return int(blockNumber), nil
}

func TestIndexing(t *testing.T) {
	// Skip this test if we don't have a real Geth connection available
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test - SKIP_INTEGRATION_TESTS is set")
	}

	logger.InitConsoleOnly(true)

	ctx := context.Background()
	
	// Use app-sdk to start defra instance for testing
	appConfig := appdefra.DefaultConfig
	schemaApplier := &appdefra.SchemaApplierFromFile{DefaultPath: "schema/schema.graphql"}
	indexerDefra, err := appdefra.StartDefraInstanceWithTestConfig(t, appConfig, schemaApplier, "Block", "Transaction", "AccessListEntry", "Log")
	require.NoError(t, err)
	defer indexerDefra.Close(ctx)

	port := defra.GetPort(indexerDefra)
	require.NotEqual(t, -1, port, "Unable to retrieve indexer's defra port")

	// Get the actual API URL from the defra node
	defraUrl := indexerDefra.APIURL

	// Create test config with dynamic port and GCP endpoint
	// Use the actual defra URL (can be LAN IP or localhost)
	testCfg := &config.Config{}
	*testCfg = *DefaultConfig // Copy the config
	testCfg.ShinzoAppConfig.DefraDB.Url = defraUrl

	i := CreateIndexer(testCfg)
	go func() {
		err := i.StartIndexing(true) // Use embedded=true to prevent conflicts
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting defra dependency: %v", err))
		}
	}()

	for !i.IsStarted() || !i.HasIndexedAtLeastOneBlock() {
		time.Sleep(100 * time.Millisecond)
	}

	blockNumber, err := queryBlockNumberFromUrl(ctx, defraUrl)
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}
