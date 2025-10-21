package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/types"
	"github.com/sourcenetwork/defradb/http"
	"github.com/sourcenetwork/defradb/node"
	"github.com/stretchr/testify/require"
)

func TestIndexing_StartDefraFirst(t *testing.T) {
	// Skip this test if we don't have a real Geth connection available
	// This test requires actual blockchain connectivity
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test - SKIP_INTEGRATION_TESTS is set")
	}

	logger.Init(true)

	defraUrl := "127.0.0.1:0"
	options := []node.Option{
		node.WithDisableAPI(false),
		node.WithDisableP2P(true),
		node.WithStorePath(t.TempDir()),
		http.WithAddress(defraUrl),
	}
	ctx := context.Background()
	indexerDefra := startDefraInstance(t, ctx, options)
	defer indexerDefra.Close(ctx)

	port := defra.GetPort(indexerDefra)
	require.NotEqual(t, -1, port, "Unable to retrieve indexer's defra port")

	_, err := queryBlockNumber(ctx, port)
	require.Error(t, err)

	// Create test config by copying DefaultConfig and updating the URL
	testCfg := &config.Config{}
	*testCfg = *DefaultConfig // Copy the config
	testCfg.DefraDB.Url = fmt.Sprintf("http://localhost:%d", port)

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

	blockNumber, err := queryBlockNumber(ctx, port)
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}

func startDefraInstance(t *testing.T, ctx context.Context, options []node.Option) *node.Node {
	myNode, err := node.New(ctx, options...)
	require.NoError(t, err)
	require.NotNil(t, myNode)

	err = myNode.Start(ctx)
	require.NoError(t, err)

	err = applySchema(ctx, myNode)
	require.NoError(t, err)

	err = defra.WaitForDefraDB(myNode.APIURL)
	require.NoError(t, err)

	return myNode
}

func queryBlockNumber(ctx context.Context, port int) (int, error) {
	handler, err := defra.NewBlockHandler(fmt.Sprintf("http://localhost:%d", port))
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

	logger.Init(true)

	// Create embedded DefraDB with dynamic port to avoid conflicts
	defraUrl := "127.0.0.1:0"
	options := []node.Option{
		node.WithDisableAPI(false),
		node.WithDisableP2P(true),
		node.WithStorePath(t.TempDir()),
		http.WithAddress(defraUrl),
	}
	ctx := context.Background()
	indexerDefra := startDefraInstance(t, ctx, options)
	defer indexerDefra.Close(ctx)

	port := defra.GetPort(indexerDefra)
	require.NotEqual(t, -1, port, "Unable to retrieve indexer's defra port")

	// Create test config with dynamic port and GCP endpoint
	testCfg := &config.Config{}
	*testCfg = *DefaultConfig // Copy the config
	testCfg.DefraDB.Url = fmt.Sprintf("http://localhost:%d", port)

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

	blockNumber, err := queryBlockNumber(ctx, port)
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}
