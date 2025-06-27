//go:build integration
// +build integration

package integration

import (
	"testing"
)

func getLatestBlockNumber(t *testing.T) int {
	query := `query { Block(order: {number: DESC}, limit: 1) { number } }`
	result := postGraphQLQuery(t, query, nil)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok || len(blockList) == 0 {
		t.Fatalf("No blocks returned: %v", result)
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
	query := `query { Block(order: {number: DESC}, limit: 10) { hash number parentHash difficulty gasUsed gasLimit nonce miner size stateRoot transactionsRoot receiptsRoot extraData } }`
	result := postGraphQLQuery(t, query, nil)
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

	query := `query($blockNumber: Int!) { Block(filter: {number: {_eq: $blockNumber}}) { hash number parentHash difficulty gasUsed gasLimit nonce miner size stateRoot transactionsRoot receiptsRoot extraData transactions { hash blockHash blockNumber from to value gasPrice inputData nonce transactionIndex logs { address topics data blockNumber transactionHash transactionIndex blockHash logIndex removed } } } }`
	variables := map[string]interface{}{ "blockNumber": blockNumber }
	result := postGraphQLQuery(t, query, variables)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok || len(blockList) == 0 {
		t.Fatalf("No block with number %v found; cannot test transactions.", blockNumber)
	}
	block := blockList[0].(map[string]interface{})
	if _, ok := block["transactions"]; !ok {
		t.Errorf("Block missing transactions field")
	}
}
