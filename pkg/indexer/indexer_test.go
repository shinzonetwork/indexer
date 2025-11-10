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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndexing_StartDefraFirst is now replaced by mock-based integration tests
// See ./integration/ directory for comprehensive integration tests with mock data
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
	t.Skip("This test has been replaced by mock-based integration tests in ./integration/ - run 'make test' for full test suite")
}

// TestCreateIndexer tests the indexer creation
func TestCreateIndexer(t *testing.T) {
	cfg := &config.Config{
		ShinzoAppConfig: appdefra.DefaultConfig,
		Indexer: config.IndexerConfig{
			StartHeight: 100,
		},
	}

	indexer := CreateIndexer(cfg)

	assert.NotNil(t, indexer)
	assert.Equal(t, cfg, indexer.cfg)
	assert.False(t, indexer.shouldIndex)
	assert.False(t, indexer.isStarted)
	assert.False(t, indexer.hasIndexedAtLeastOneBlock)
	assert.Nil(t, indexer.defraNode)
}

// TestIndexerStateManagement tests the state management methods
func TestIndexerStateManagement(t *testing.T) {
	cfg := &config.Config{
		ShinzoAppConfig: appdefra.DefaultConfig,
	}
	indexer := CreateIndexer(cfg)

	// Test initial state
	assert.False(t, indexer.IsStarted())
	assert.False(t, indexer.HasIndexedAtLeastOneBlock())

	// Test state changes
	indexer.shouldIndex = true
	indexer.isStarted = true
	indexer.hasIndexedAtLeastOneBlock = true

	assert.True(t, indexer.IsStarted())
	assert.True(t, indexer.HasIndexedAtLeastOneBlock())
}

// TestStopIndexing tests the stop indexing functionality
func TestStopIndexing(t *testing.T) {
	cfg := &config.Config{
		ShinzoAppConfig: appdefra.DefaultConfig,
	}
	indexer := CreateIndexer(cfg)

	// Set some state
	indexer.shouldIndex = true
	indexer.isStarted = true
	indexer.hasIndexedAtLeastOneBlock = true

	// Stop indexing
	indexer.StopIndexing()

	// Verify state is reset
	assert.False(t, indexer.shouldIndex)
	assert.False(t, indexer.isStarted)
	// hasIndexedAtLeastOneBlock should remain true (historical fact)
	assert.True(t, indexer.hasIndexedAtLeastOneBlock)
}

// TestGetEnvOrDefault tests the environment variable helper function
func TestGetEnvOrDefault(t *testing.T) {
	// Test with non-existent env var
	result := getEnvOrDefault("NON_EXISTENT_VAR", "default_value")
	assert.Equal(t, "default_value", result)

	// Test with existing env var
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	result = getEnvOrDefault("TEST_VAR", "default_value")
	assert.Equal(t, "test_value", result)
}

// TestConstants tests the defined constants
func TestConstants(t *testing.T) {
	assert.Equal(t, 10, DefaultBlocksToIndexAtOnce)
	assert.Equal(t, 3, DefaultRetryAttempts)
	assert.Equal(t, 15*time.Second, DefaultSchemaWaitTimeout)
	assert.Equal(t, 30*time.Second, DefaultDefraReadyTimeout)
	assert.Equal(t, 3, DefaultBlockOffset)
	assert.Equal(t, "/ip4/127.0.0.1/tcp/9171", defaultListenAddress)
}

// TestConvertGethBlockToDefraBlock tests block conversion
func TestConvertGethBlockToDefraBlock(t *testing.T) {
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
