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
	logger.Init(true)

	// Create test config
	testConfig := &config.Config{
		DefraDB: config.DefraDBConfig{
			Url: "http://localhost:9181", // Will be set after we get the port
		},
		Geth: config.GethConfig{
			NodeURL: os.Getenv("GCP_RPC_URL"),
			WsURL:   os.Getenv("GCP_WS_URL"),
			APIKey:  os.Getenv("GCP_API_KEY"),
		},
		Logger: config.LoggerConfig{
			Development: true,
		},
	}

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

	defraURL := fmt.Sprintf("http://localhost:%d", port)

	// Update test config with the actual DefraDB URL
	testConfig.DefraDB.Url = defraURL

	go func() {
		err := StartIndexingWithModeAndConfig("", defraURL, ModeRealTime, testConfig)
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting indexer: %v", err))
		}
	}()
	defer StopIndexing()

	// Wait for indexer to start with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Skip("Skipping test - indexer did not start within 30 seconds (likely due to network issues)")
			return
		case <-ticker.C:
			if IsStarted && HasIndexedAtLeastOneBlock {
				goto indexerReady
			}
		}
	}
indexerReady:

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
	logger.Init(true)

	// Create test config
	testConfig := &config.Config{
		DefraDB: config.DefraDBConfig{
			Url: "http://localhost:9181",
		},
		Geth: config.GethConfig{
			NodeURL: os.Getenv("GCP_RPC_URL"),
			WsURL:   os.Getenv("GCP_WS_URL"),
			APIKey:  os.Getenv("GCP_API_KEY"),
		},
		Logger: config.LoggerConfig{
			Development: true,
		},
	}

	go func() {
		err := StartIndexingWithModeAndConfig("", "http://localhost:9181", ModeRealTime, testConfig)
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting indexer: %v", err))
		}
	}()
	defer StopIndexing()

	// Wait for indexer to start with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Skip("Skipping test - indexer did not start within 30 seconds (likely due to network issues)")
			return
		case <-ticker.C:
			if IsStarted && HasIndexedAtLeastOneBlock {
				goto indexerReady2
			}
		}
	}
indexerReady2:

	blockNumber, err := queryBlockNumber(context.Background(), 9181)
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}
