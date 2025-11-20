package integration

import (
	"path/filepath"
	"testing"
)

const queryFile = "queries/blocks.graphql"

var blockQueryPath string

func init() {
	blockQueryPath = filepath.Join(getProjectRoot(nil), queryFile)
}

// Helper to get the latest block number
func getLatestBlockNumber(t *testing.T) int {
	result := MakeQuery(t, blockQueryPath, "GetHighestBlockNumber", nil)

	// Check if result is nil
	if result == nil {
		t.Fatalf("GraphQL query returned nil result")
	}

	// Check if data field exists
	data, ok := result["data"]
	if !ok {
		t.Fatalf("No 'data' field in GraphQL response: %v", result)
	}

	// Check if data is nil
	if data == nil {
		t.Fatalf("GraphQL 'data' field is nil: %v", result)
	}

	// Cast data to map
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("GraphQL 'data' field is not a map: %v", data)
	}

	// Check if Block field exists
	blockField, ok := dataMap["Block"]
	if !ok {
		t.Fatalf("No 'Block' field in GraphQL data: %v", dataMap)
	}

	// Cast Block to array
	blockList, ok := blockField.([]interface{})
	if !ok {
		t.Fatalf("Block field is not an array: %v", blockField)
	}

	if len(blockList) == 0 {
		t.Fatalf("No blocks returned from DefraDB - database may be empty")
	}

	num, ok := blockList[0].(map[string]interface{})["number"]
	if !ok {
		t.Fatalf("Block missing number field: %v", blockList[0])
	}
	n, ok := num.(float64)
	if !ok {
		t.Fatalf("Block number is not a number: %v", num)
	}
	return int(n)
}

func TestGetHighestBlockNumber(t *testing.T) {
	_ = getLatestBlockNumber(t) // Just check we can get it
}

func TestGetLatestBlocks(t *testing.T) {
	result := MakeQuery(t, blockQueryPath, "GetLatestBlocks", nil)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok {
		t.Fatalf("No Block field or wrong type in response: %v", result)
	}
	if len(blockList) == 0 {
		t.Fatalf("No blocks returned")
	}
	for _, b := range blockList {
		block := b.(map[string]interface{})
		if _, ok := block["hash"]; !ok {
			t.Errorf("Block missing hash field")
		}
		if _, ok := block["number"]; !ok {
			t.Errorf("Block missing number field")
		}
	}
}

func TestGetBlockWithTransactions(t *testing.T) {
	blockNumber := getLatestBlockNumber(t)
	variables := map[string]interface{}{"blockNumber": blockNumber}
	result := MakeQuery(t, blockQueryPath, "GetBlockWithTransactions", variables)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok || len(blockList) == 0 {
		t.Fatalf("No block with number %v found; cannot test transactions.", blockNumber)
	}
	block := blockList[0].(map[string]interface{})
	if _, ok := block["transactions"]; !ok {
		t.Errorf("Block missing transactions field")
	}
}
