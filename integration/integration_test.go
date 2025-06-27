//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

const graphqlURL = "http://localhost:9181/api/v0/graphql"

func TestIntegration_Connection(t *testing.T) {
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewBuffer([]byte(`{"query":"query { __typename }"}`)))
	if err != nil {
		t.Fatalf("Failed to connect to GraphQL endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if _, ok := result["data"]; !ok {
		t.Fatalf("No data field in response: %s", string(body))
	}
}

func postGraphQLQuery(t *testing.T, query string, variables map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		t.Fatalf("Failed to POST query: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return result
}

func TestIntegration_GetHighestBlockNumber(t *testing.T) {
	query := `query { Block(order: {number: DESC}, limit: 1) { number } }`
	result := postGraphQLQuery(t, query, nil)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok {
		t.Fatalf("No Block field or wrong type in response: %v", result)
	}
	if len(blockList) == 0 {
		t.Fatalf("No blocks returned")
	}
	if _, ok := blockList[0].(map[string]interface{})["number"]; !ok {
		t.Fatalf("Block missing number field")
	}
}

func TestIntegration_GetLatestBlocks(t *testing.T) {
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

func TestIntegration_GetBlockWithTransactions(t *testing.T) {
	query := `query($blockNumber: Int!) { Block(filter: {number: {_eq: $blockNumber}}) { hash number parentHash difficulty gasUsed gasLimit nonce miner size stateRoot transactionsRoot receiptsRoot extraData transactions { hash blockHash blockNumber from to value gasPrice inputData nonce transactionIndex logs { address topics data blockNumber transactionHash transactionIndex blockHash logIndex removed } } } }`
	variables := map[string]interface{}{ "blockNumber": 1 }
	result := postGraphQLQuery(t, query, variables)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok {
		t.Fatalf("No Block field or wrong type in response: %v", result)
	}
	if len(blockList) == 0 {
		t.Skip("No block with number 1 found; skipping test.")
	}
	block := blockList[0].(map[string]interface{})
	if _, ok := block["transactions"]; !ok {
		t.Errorf("Block missing transactions field")
	}
} 