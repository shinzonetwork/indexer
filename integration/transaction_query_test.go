package integration

import (
	"path/filepath"
	"testing"
)

var transactionQueryPath string

func init() {
	// Initialize queryPath once for all tests
	transactionQueryPath = filepath.Join(getProjectRoot(nil), "queries/transaction.graphql")
}

func getArbitraryTransactionHash(t *testing.T) string {
	blockNumber := getLatestBlockNumber(t)
	blockNumber = blockNumber - 25 // Latest blocks returned by Alchemy often do not have transactions
	if blockNumber < 0 {
		t.Fatalf("Block number underflow: %d", blockNumber)
	}
	variables := map[string]interface{}{"blockNumber": blockNumber}
	result := MakeQuery(t, blockQueryPath, "GetBlockWithTransactions", variables)
	blockList, ok := result["data"].(map[string]interface{})["Block"].([]interface{})
	if !ok || len(blockList) == 0 {
		t.Fatalf("No block with number %v found; cannot test transactions.", blockNumber)
	}
	block := blockList[0].(map[string]interface{})
	transactions, ok := block["transactions"].([]interface{})
	if !ok || len(transactions) == 0 {
		t.Fatalf("No transactions found in block %d: %v", blockNumber, result)
	}
	firstTx, ok := transactions[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Transaction is not a map: %v", transactions[0])
	}
	hash, ok := firstTx["hash"].(string)
	if !ok || len(hash) == 0 {
		t.Fatalf("Transaction hash missing or empty: %v", firstTx)
	}
	return hash
}

func TestGetTransactionByHash(t *testing.T) {
	transactionHash := getArbitraryTransactionHash(t)
	result := MakeQuery(t, transactionQueryPath, "GetTransactionByHash", map[string]interface{}{"txHash": transactionHash})
	transactionList, ok := result["data"].(map[string]interface{})["Transaction"].([]interface{})
	if !ok || len(transactionList) == 0 {
		t.Errorf("No transactions returned: %v", result)
		return
	}
	hash, ok := transactionList[0].(map[string]interface{})["hash"]
	if !ok {
		t.Errorf("Transaction missing hash field: %v", transactionList[0])
		return
	}
	hashStr, ok := hash.(string)
	if !ok {
		t.Errorf("Transaction hash is not a string: %v", hash)
	} else if len(hashStr) == 0 {
		t.Errorf("Got empty hash: %v", transactionList[0])
	}

	if hashStr != transactionHash {
		t.Errorf("Transaction returned doesn't match our transaction hash input: received %v ; given %v", transactionList, transactionHash)
	}
}
