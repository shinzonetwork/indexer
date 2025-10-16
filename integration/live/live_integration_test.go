//go:build live
// +build live

package live

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/indexer"
	"github.com/shinzonetwork/indexer/pkg/logger"
)

const (
	liveGraphqlURL = "http://localhost:9181/api/v0/graphql" // Different port to avoid conflicts
	liveDefraURL   = "http://localhost:9181"
)

var (
	indexerStarted   = false
	indexerCtx       context.Context
	indexerCancel    context.CancelFunc
	liveChainIndexer *indexer.ChainIndexer
)

// TestMain sets up and tears down the live integration test environment
func TestMain(m *testing.M) {
	// Initialize logger for live integration tests first
	logger.Init(true)
	logger.Test("TestMain - Starting live integration tests with real Ethereum data")

	// Check required environment variables
	if !checkRequiredEnvVars() {
		logger.Sugar.Error("Required environment variables not set. Set GCP_GETH_RPC_URL, GCP_GETH_WS_URL, and GCP_GETH_API_KEY")
		os.Exit(0) // treat as skipped instead of failed
	}

	// Clean up any existing live integration DefraDB data
	logger.Test("Cleaning up existing live integration DefraDB data...")
	if err := os.RemoveAll("./integration/.defra"); err != nil {
		logger.Sugar.Warnf("Failed to clean existing live data: %v", err)
	}

	// Set DefraDB port for live tests to avoid conflicts
	os.Setenv("DEFRADB_PORT", "9181")

	// Start live indexer with real Ethereum connections
	logger.Test("Starting live indexer with real Ethereum connections...")
	indexerCtx, indexerCancel = context.WithCancel(context.Background())
	go func() {
		// Load config for live testing
		cfg, err := config.LoadConfig("../../config/test.yaml")
		if err != nil {
			logger.Sugar.Errorf("Failed to load config: %v", err)
			return
		}
		
		// Override DefraDB store path for live testing
		cfg.DefraDB.Store.Path = "../.defra"
		
		// Override Geth config with environment variables for live testing
		cfg.Geth.NodeURL = os.Getenv("GCP_GETH_RPC_URL")
		cfg.Geth.WsURL = os.Getenv("GCP_GETH_WS_URL")
		cfg.Geth.APIKey = os.Getenv("GCP_GETH_API_KEY")

		// Start indexer with real connections - should succeed if env vars are set
		liveChainIndexer = indexer.CreateIndexer(cfg)
		err = liveChainIndexer.StartIndexing(false) // false = start embedded DefraDB
		if err != nil {
			logger.Sugar.Errorf("Live indexer failed: %v", err)
		}
	}()

	// Wait for live DefraDB to be ready
	logger.Test("Waiting for live DefraDB to be ready...")
	if !waitForLiveDefraDB(30 * time.Second) {
		logger.Sugar.Error("Failed to start live DefraDB - skipping live integration tests")
		os.Exit(1)
	}
	logger.Test("Live DefraDB is ready!")
	indexerStarted = true

	// Wait for some live blocks to be indexed
	logger.Test("Waiting for live blocks to be indexed...")
	if !waitForLiveBlocks(60 * time.Second) {
		logger.Sugar.Warn("No live blocks indexed within timeout - tests may fail")
	}

	// Run tests
	result := m.Run()

	// Teardown
	logger.Test("TestMain - Live integration teardown")
	if liveChainIndexer != nil {
		liveChainIndexer.StopIndexing()
	}

	// Clean up test data
	time.Sleep(2 * time.Second) // Give time for cleanup
	os.RemoveAll("../.defra")

	os.Exit(result)
}

// checkRequiredEnvVars checks if all required environment variables are set for live testing
func checkRequiredEnvVars() bool {
	requiredVars := []string{"GCP_GETH_RPC_URL", "GCP_GETH_WS_URL", "GCP_GETH_API_KEY"}

	for _, envVar := range requiredVars {
		if os.Getenv(envVar) == "" {
			logger.Sugar.Errorf("❌ Missing required environment variable: %s", envVar)
			return false
		}
	}

	logger.Test("✓ All required environment variables are set")
	return true
}

// waitForLiveDefraDB waits for DefraDB to be ready
func waitForLiveDefraDB(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if testLiveDefraDBConnection() {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

// testLiveDefraDBConnection tests if DefraDB is responding
func testLiveDefraDBConnection() bool {
	resp, err := http.Get(liveDefraURL + "/api/v0/schema")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// waitForLiveBlocks waits for live blocks to be indexed
func waitForLiveBlocks(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hasLiveBlocks() {
			return true
		}
		time.Sleep(5 * time.Second) // Check every 5 seconds for live data
	}
	return false
}

// hasLiveBlocks checks if any blocks have been indexed from live Ethereum
func hasLiveBlocks() bool {
	query := `{"query":"query { Block(limit: 1) { number hash } }"}`
	resp, err := http.Post(liveGraphqlURL, "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return false
	}

	blocks, ok := data["Block"].([]interface{})
	return ok && len(blocks) > 0
}

// TestLiveEthereumConnection tests that the indexer can connect to real Ethereum
func TestLiveEthereumConnection(t *testing.T) {
	if !indexerStarted {
		t.Skip("Live indexer not started - skipping live tests")
	}

	logger.Test("Testing live Ethereum connection and block indexing")

	// Check that we have live blocks
	if !hasLiveBlocks() {
		t.Fatal("No live blocks found - indexer may not be connected to Ethereum")
	}

	logger.Test("✓ Live Ethereum connection successful - blocks are being indexed")
}

// TestLiveGetLatestBlocks tests querying latest blocks from live data
func TestLiveGetLatestBlocks(t *testing.T) {
	if !indexerStarted {
		t.Skip("Live indexer not started - skipping live tests")
	}

	query := `{"query":"query { Block(limit: 5, order: {number: DESC}) { number hash timestamp gasUsed gasLimit miner } }"}`

	resp, err := http.Post(liveGraphqlURL, "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		t.Fatalf("Failed to query live blocks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GraphQL query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("No data in response")
	}

	blocks, ok := data["Block"].([]interface{})
	if !ok || len(blocks) == 0 {
		t.Fatal("No blocks returned from live query")
	}

	logger.Testf("✓ Successfully queried %d live blocks", len(blocks))

	// Validate block structure
	firstBlock := blocks[0].(map[string]interface{})
	requiredFields := []string{"number", "hash", "timestamp", "gasUsed", "gasLimit", "miner"}

	for _, field := range requiredFields {
		if _, exists := firstBlock[field]; !exists {
			t.Errorf("Missing required field '%s' in live block", field)
		}
	}
}

// TestLiveGetTransactions tests querying transactions from live data
func TestLiveGetTransactions(t *testing.T) {
	if !indexerStarted {
		t.Skip("Live indexer not started - skipping live tests")
	}

	query := `{"query":"query { Transaction(limit: 3) { hash blockNumber from to value gas gasPrice gasUsed status } }"}`

	resp, err := http.Post(liveGraphqlURL, "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		t.Fatalf("Failed to query live transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Transaction query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode transaction response: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("No data in transaction response")
	}

	transactions, ok := data["Transaction"].([]interface{})
	if !ok {
		logger.Test("No transactions found in live data (this may be normal if blocks have no transactions)")
		return
	}

	logger.Testf("✓ Successfully queried %d live transactions", len(transactions))

	if len(transactions) > 0 {
		// Validate transaction structure
		firstTx := transactions[0].(map[string]interface{})
		requiredFields := []string{"hash", "blockNumber", "from", "to", "value", "gas", "gasPrice"}

		for _, field := range requiredFields {
			if _, exists := firstTx[field]; !exists {
				t.Errorf("Missing required field '%s' in live transaction", field)
			}
		}
	}
}

// TestLiveBlockTransactionRelationship tests the relationship between blocks and transactions in live data
func TestLiveBlockTransactionRelationship(t *testing.T) {
	if !indexerStarted {
		t.Skip("Live indexer not started - skipping live tests")
	}

	// Get a block with its transactions
	query := `{"query":"query { Block(limit: 1, filter: {}) { number hash transactions { hash blockNumber from to } } }"}`

	resp, err := http.Post(liveGraphqlURL, "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		t.Fatalf("Failed to query live block with transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Block-transaction query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode block-transaction response: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("No data in block-transaction response")
	}

	blocks, ok := data["Block"].([]interface{})
	if !ok || len(blocks) == 0 {
		t.Fatal("No blocks returned from live block-transaction query")
	}

	block := blocks[0].(map[string]interface{})
	blockNumber := block["number"]

	// Check if block has transactions
	transactions, hasTransactions := block["transactions"].([]interface{})
	if !hasTransactions {
		logger.Test("Block has no transactions (this may be normal)")
		return
	}

	logger.Testf("✓ Block %v has %d transactions", blockNumber, len(transactions))

	// Validate that transaction blockNumbers match the block number
	for i, tx := range transactions {
		txMap := tx.(map[string]interface{})
		txBlockNumber := txMap["blockNumber"]

		if txBlockNumber != blockNumber {
			t.Errorf("Transaction %d has blockNumber %v but belongs to block %v", i, txBlockNumber, blockNumber)
		}
	}
}

// TestLiveIndexerPerformance tests the performance of live indexing
func TestLiveIndexerPerformance(t *testing.T) {
	if !indexerStarted {
		t.Skip("Live indexer not started - skipping live tests")
	}

	// Get current block count
	initialCount := getLiveBlockCount()
	if initialCount == 0 {
		t.Skip("No blocks indexed yet - skipping performance test")
	}

	logger.Testf("Initial block count: %d", initialCount)

	// Wait for more blocks to be indexed
	time.Sleep(30 * time.Second)

	finalCount := getLiveBlockCount()
	logger.Testf("Final block count: %d", finalCount)

	if finalCount <= initialCount {
		logger.Test("Warning: No new blocks indexed during test period (this may be normal depending on network activity)")
	} else {
		blocksIndexed := finalCount - initialCount
		logger.Testf("✓ Indexed %d new blocks in 30 seconds", blocksIndexed)
	}
}

// getLiveBlockCount returns the total number of blocks indexed
func getLiveBlockCount() int {
	query := `{"query":"query { Block { _count } }"}`
	resp, err := http.Post(liveGraphqlURL, "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return 0
	}

	blocks, ok := data["Block"].([]interface{})
	if !ok {
		return 0
	}

	return len(blocks)
}
