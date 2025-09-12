package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/types"
	"github.com/sourcenetwork/defradb/http"
	"github.com/sourcenetwork/defradb/node"
	"github.com/stretchr/testify/require"
)

func TestIndexing_StartDefraFirst(t *testing.T) {
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

	testConfig := defaultConfig
	testConfig.DefraDB.Url = fmt.Sprintf("http://localhost:%d", port)

	go func() {
		err := StartIndexing(true, testConfig)
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting defra dependency: %v", err))
		}
	}()
	defer StopIndexing()

	for !IsStarted || !HasIndexedAtLeastOneBlock {
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
	logger.Init(true)

	go func() {
		err := StartIndexing(false, nil)
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting defra dependency: %v", err))
		}
	}()
	defer StopIndexing()

	for !IsStarted || !HasIndexedAtLeastOneBlock {
		time.Sleep(100 * time.Millisecond)
	}

	blockNumber, err := queryBlockNumber(context.Background(), defra.GetPortFromUrl(defaultConfig.DefraDB.Url))
	require.NoError(t, err)
	require.Greater(t, blockNumber, 100)
}
