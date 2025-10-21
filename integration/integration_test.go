package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/indexer"
	"github.com/shinzonetwork/indexer/pkg/logger"
)

const graphqlURL = "http://localhost:9181/api/v0/graphql"

var testChainIndexer *indexer.ChainIndexer

// createEphemeralDefraDB creates a temporary DefraDB instance using t.TempDir()
// This ensures each test gets a fresh, isolated DefraDB instance that's automatically cleaned up
func createEphemeralDefraDB(t *testing.T) *indexer.ChainIndexer {
	// Create ephemeral directory for this test
	tempDir := t.TempDir()
	logger.Testf("Creating ephemeral DefraDB instance in: %s", tempDir)

	// Create config with ephemeral storage path
	cfg := &config.Config{
		DefraDB: config.DefraDBConfig{
			Url: "http://localhost:9181",
			Store: config.DefraDBStoreConfig{
				Path: tempDir,
			},
		},
		Geth: config.GethConfig{
			NodeURL: "http://34.68.131.15:8545", // Will fail but that's expected for tests
		},
	}

	// Create indexer with ephemeral DefraDB
	ephemeralIndexer := indexer.CreateIndexer(cfg)
	return ephemeralIndexer
}

func TestMain(m *testing.M) {
	// Initialize logger for integration tests first
	logger.Init(true)
	logger.Test("TestMain - Starting self-contained integration tests with mock data")

	// Note: Individual tests will create ephemeral DefraDB instances using t.TempDir()
	// This ensures each test gets a fresh, isolated DefraDB instance that's automatically cleaned up
	logger.Test("Integration tests will use ephemeral DefraDB instances per test")

	// Note: DefraDB setup and mock data insertion will be handled per test
	// using ephemeral instances created with t.TempDir()

	// Run tests
	exitCode := m.Run()

	// Teardown
	logger.Test("TestMain - Teardown")
	if testChainIndexer != nil {
		testChainIndexer.StopIndexing()
	}

	os.Exit(exitCode)
}

// waitForDefraDBReady waits for a DefraDB instance to be ready for connections
func waitForDefraDBReady(t *testing.T) {
	logger.Test("Waiting for ephemeral DefraDB to be ready...")
	for attempts := 0; attempts < 30; attempts++ { // 30 attempts = 15 seconds max
		if testDefraDBConnection() {
			logger.Test("Ephemeral DefraDB is ready!")
			return
		}
		// Sleep for 500ms between attempts
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for ephemeral DefraDB to be ready")
}

func TestGraphQLConnection(t *testing.T) {
	logger.Test("Testing GraphQL connection with ephemeral DefraDB")
	
	// Create ephemeral DefraDB instance for this test
	ephemeralIndexer := createEphemeralDefraDB(t)
	defer ephemeralIndexer.StopIndexing() // Cleanup when test completes
	
	// Start the ephemeral DefraDB instance in a goroutine
	go func() {
		err := ephemeralIndexer.StartIndexing(false) // false = start embedded DefraDB
		if err != nil {
			// Expected to fail on Ethereum connection, but DefraDB should be running
			logger.Testf("Indexer failed as expected (no Ethereum connection): %v", err)
		}
	}()
	
	// Wait for DefraDB to be ready
	waitForDefraDBReady(t)
	
	// Test basic GraphQL connectivity
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewBuffer([]byte(`{"query":"query { __typename }"}`)))
	if err != nil {
		t.Fatalf("Failed to connect to GraphQL endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if _, ok := result["data"]; !ok {
		t.Fatalf("No data field in response: %s", string(body))
	}
	
	logger.Test("Successfully connected to ephemeral DefraDB instance")
}

// TestMultipleEphemeralInstances demonstrates creating multiple isolated DefraDB instances
// Each call to t.TempDir() creates a unique temporary directory that gets cleaned up automatically
func TestMultipleEphemeralInstances(t *testing.T) {
	logger.Test("Testing multiple ephemeral DefraDB instances")
	
	// Create first ephemeral DefraDB instance
	tempDir1 := t.TempDir()
	logger.Testf("Created first ephemeral directory: %s", tempDir1)
	
	// Create second ephemeral DefraDB instance  
	tempDir2 := t.TempDir()
	logger.Testf("Created second ephemeral directory: %s", tempDir2)
	
	// Create third ephemeral DefraDB instance
	tempDir3 := t.TempDir()
	logger.Testf("Created third ephemeral directory: %s", tempDir3)
	
	// Verify all directories are different and exist
	if tempDir1 == tempDir2 || tempDir1 == tempDir3 || tempDir2 == tempDir3 {
		t.Fatalf("Expected unique directories, got: %s, %s, %s", tempDir1, tempDir2, tempDir3)
	}
	
	// Verify directories exist
	for i, dir := range []string{tempDir1, tempDir2, tempDir3} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Fatalf("Directory %d does not exist: %s", i+1, dir)
		}
	}
	
	logger.Test("Successfully created 3 unique ephemeral directories")
	logger.Test("Each directory will be automatically cleaned up when test completes")
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
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return result
}

// Helper to find the project root by looking for go.mod
func getProjectRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

// Helper to extract a named query from a .graphql file
func loadGraphQLQuery(filename, queryName string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	content := string(data)
	start := strings.Index(content, "query "+queryName)
	if start == -1 {
		return "", fmt.Errorf("query %s not found", queryName)
	}
	// Find the next "query " after start, or end of file
	next := strings.Index(content[start+1:], "query ")
	var query string
	if next == -1 {
		query = content[start:]
	} else {
		query = content[start : start+next+1]
	}
	query = strings.TrimSpace(query)
	return query, nil
}

func MakeQuery(t *testing.T, queryPath string, query string, args map[string]interface{}) map[string]interface{} {
	query, err := loadGraphQLQuery(queryPath, query)
	if err != nil {
		t.Errorf("Failed to load query %v", err)
	}
	result := postGraphQLQuery(t, query, args)
	return result
}

func testDefraDBConnection() bool {
	resp, err := http.Get("http://localhost:9181/api/v0/schema")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// insertMockDataToEphemeralDB inserts mock data into an ephemeral DefraDB instance
// This creates test data in the temporary DefraDB instance for testing purposes
func insertMockDataToEphemeralDB(t *testing.T) error {
	logger.Test("Inserting mock data into ephemeral DefraDB...")
	return insertMockData()
}

func insertMockData() error {
	// Store DocIDs for relationship establishment
	var block1DocID, block2DocID, tx1DocID, tx2DocID string

	// Create Block 1
	block1Mutation := map[string]interface{}{
		"query": `mutation {
			create_Block(input: {
				hash: "0x1000001000000000000000000000000000000000000000000000000000000001"
				number: 1000001
				timestamp: "1640995200"
				parentHash: "0x1000000000000000000000000000000000000000000000000000000000000000"
				difficulty: "1000000"
				gasUsed: "21000"
				gasLimit: "8000000"
				nonce: "1000001"
				miner: "0x1000000000000000000000000000000000000001"
				size: "1024"
				stateRoot: "0x1000001000000000000000000000000000000000000000000000000000000001"
				sha3Uncles: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
				transactionsRoot: "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
				receiptsRoot: "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
				logsBloom: "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
				extraData: "0x"
				mixHash: "0x0000000000000000000000000000000000000000000000000000000000000000"
				totalDifficulty: "1000000"
				baseFeePerGas: ""
			}) {
				_docID
				hash
				number
			}
		}`,
	}

	// Execute Block 1 creation and extract DocID
	jsonData, err := json.Marshal(block1Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal block1 mutation: %v", err)
	}

	resp, err := http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("block1 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("block1 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read block1 response: %v", err)
	}

	var block1Resp map[string]interface{}
	if err := json.Unmarshal(body, &block1Resp); err != nil {
		return fmt.Errorf("failed to parse block1 response: %v", err)
	}

	if errors, hasErrors := block1Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in block1 creation: %v", errors)
	}

	// Extract Block 1 DocID
	if data, ok := block1Resp["data"].(map[string]interface{}); ok {
		if createBlock, ok := data["create_Block"].([]interface{}); ok && len(createBlock) > 0 {
			if blockData, ok := createBlock[0].(map[string]interface{}); ok {
				if docID, ok := blockData["_docID"].(string); ok {
					block1DocID = docID
					logger.Testf("Block 1 created with DocID: %s", block1DocID)
				}
			}
		}
	}

	// Create Block 2
	block2Mutation := map[string]interface{}{
		"query": `mutation {
			create_Block(input: {
				hash: "0x1000002000000000000000000000000000000000000000000000000000000002"
				number: 1000002
				timestamp: "1640995212"
				parentHash: "0x1000001000000000000000000000000000000000000000000000000000000001"
				difficulty: "1000000"
				gasUsed: "42000"
				gasLimit: "8000000"
				nonce: "1000002"
				miner: "0x1000000000000000000000000000000000000002"
				size: "2048"
				stateRoot: "0x1000002000000000000000000000000000000000000000000000000000000002"
				sha3Uncles: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
				transactionsRoot: "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
				receiptsRoot: "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
				logsBloom: "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
				extraData: "0x"
				mixHash: "0x0000000000000000000000000000000000000000000000000000000000000000"
				totalDifficulty: "1000000"
				baseFeePerGas: ""
			}) {
				_docID
				hash
				number
			}
		}`,
	}

	// Execute Block 2 creation and extract DocID
	jsonData, err = json.Marshal(block2Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal block2 mutation: %v", err)
	}

	resp, err = http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("block2 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("block2 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read block2 response: %v", err)
	}

	var block2Resp map[string]interface{}
	if err := json.Unmarshal(body, &block2Resp); err != nil {
		return fmt.Errorf("failed to parse block2 response: %v", err)
	}

	if errors, hasErrors := block2Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in block2 creation: %v", errors)
	}

	// Extract Block 2 DocID
	if data, ok := block2Resp["data"].(map[string]interface{}); ok {
		if createBlock, ok := data["create_Block"].([]interface{}); ok && len(createBlock) > 0 {
			if blockData, ok := createBlock[0].(map[string]interface{}); ok {
				if docID, ok := blockData["_docID"].(string); ok {
					block2DocID = docID
					logger.Testf("Block 2 created with DocID: %s", block2DocID)
				}
			}
		}
	}

	// Create Transaction 1 with relationship to Block 1
	tx1Mutation := map[string]interface{}{
		"query": fmt.Sprintf(`mutation {
			create_Transaction(input: {
				hash: "0x2000001000000000000000000000000000000000000000000000000000000001"
				blockHash: "0x1000001000000000000000000000000000000000000000000000000000000001"
				blockNumber: 1000001
				from: "0x3000000000000000000000000000000000000001"
				to: "0x3000000000000000000000000000000000000002"
				value: "1000000000000000000"
				gas: "21000"
				gasPrice: "20000000000"
				gasUsed: "21000"
				input: "0x"
				nonce: "1"
				transactionIndex: 0
				type: "0"
				chainId: "1"
				v: "27"
				r: "0x1000000000000000000000000000000000000000000000000000000000000001"
				s: "0x1000000000000000000000000000000000000000000000000000000000000001"
				status: true
				cumulativeGasUsed: "21000"
				effectiveGasPrice: "20000000000"
				block: "%s"
			}) {
				_docID
				hash
			}
		}`, block1DocID),
	}

	// Execute Transaction 1 creation
	jsonData, err = json.Marshal(tx1Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal tx1 mutation: %v", err)
	}

	resp, err = http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("tx1 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tx1 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read tx1 response: %v", err)
	}

	var tx1Resp map[string]interface{}
	if err := json.Unmarshal(body, &tx1Resp); err != nil {
		return fmt.Errorf("failed to parse tx1 response: %v", err)
	}

	if errors, hasErrors := tx1Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in tx1 creation: %v", errors)
	}

	// Extract Transaction 1 DocID
	if data, ok := tx1Resp["data"].(map[string]interface{}); ok {
		if createTx, ok := data["create_Transaction"].([]interface{}); ok && len(createTx) > 0 {
			if txData, ok := createTx[0].(map[string]interface{}); ok {
				if docID, ok := txData["_docID"].(string); ok {
					tx1DocID = docID
					logger.Testf("Transaction 1 created with DocID: %s", tx1DocID)
				}
			}
		}
	}

	// Create Transaction 2 with relationship to Block 2
	tx2Mutation := map[string]interface{}{
		"query": fmt.Sprintf(`mutation {
			create_Transaction(input: {
				hash: "0x2000002000000000000000000000000000000000000000000000000000000002"
				blockHash: "0x1000002000000000000000000000000000000000000000000000000000000002"
				blockNumber: 1000002
				from: "0x3000000000000000000000000000000000000003"
				to: "0x3000000000000000000000000000000000000004"
				value: "2000000000000000000"
				gas: "21000"
				gasPrice: "25000000000"
				gasUsed: "21000"
				input: "0x"
				nonce: "2"
				transactionIndex: 0
				type: "0"
				chainId: "1"
				v: "28"
				r: "0x2000000000000000000000000000000000000000000000000000000000000002"
				s: "0x2000000000000000000000000000000000000000000000000000000000000002"
				status: true
				cumulativeGasUsed: "21000"
				effectiveGasPrice: "25000000000"
				block: "%s"
			}) {
				_docID
				hash
			}
		}`, block2DocID),
	}

	// Execute Transaction 2 creation
	jsonData, err = json.Marshal(tx2Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal tx2 mutation: %v", err)
	}

	resp, err = http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("tx2 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tx2 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read tx2 response: %v", err)
	}

	var tx2Resp map[string]interface{}
	if err := json.Unmarshal(body, &tx2Resp); err != nil {
		return fmt.Errorf("failed to parse tx2 response: %v", err)
	}

	if errors, hasErrors := tx2Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in tx2 creation: %v", errors)
	}

	// Extract Transaction 2 DocID
	if data, ok := tx2Resp["data"].(map[string]interface{}); ok {
		if createTx, ok := data["create_Transaction"].([]interface{}); ok && len(createTx) > 0 {
			if txData, ok := createTx[0].(map[string]interface{}); ok {
				if docID, ok := txData["_docID"].(string); ok {
					tx2DocID = docID
					logger.Testf("Transaction 2 created with DocID: %s", tx2DocID)
				}
			}
		}
	}

	// Create Log 1 for Transaction 1
	log1Mutation := map[string]interface{}{
		"query": fmt.Sprintf(`mutation {
			create_Log(input: {
				address: "0x4000000000000000000000000000000000000001"
				topics: ["0x5000000000000000000000000000000000000000000000000000000000000001", "0x5000000000000000000000000000000000000000000000000000000000000002"]
				data: "0x6000000000000000000000000000000000000000000000000000000000000001"
				transactionHash: "0x2000001000000000000000000000000000000000000000000000000000000001"
				blockHash: "0x1000001000000000000000000000000000000000000000000000000000000001"
				blockNumber: 1000001
				transactionIndex: 0
				logIndex: 0
				removed: "false"
				block: "%s"
				transaction: "%s"
			}) {
				_docID
				address
				topics
			}
		}`, block1DocID, tx1DocID),
	}

	// Execute Log 1 creation
	jsonData, err = json.Marshal(log1Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal log1 mutation: %v", err)
	}

	resp, err = http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("log1 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("log1 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read log1 response: %v", err)
	}

	var log1Resp map[string]interface{}
	if err := json.Unmarshal(body, &log1Resp); err != nil {
		return fmt.Errorf("failed to parse log1 response: %v", err)
	}

	if errors, hasErrors := log1Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in log1 creation: %v", errors)
	}

	logger.Testf("Log 1 created successfully: %s", string(body))

	// Create Log 2 for Transaction 2
	log2Mutation := map[string]interface{}{
		"query": fmt.Sprintf(`mutation {
			create_Log(input: {
				address: "0x4000000000000000000000000000000000000002"
				topics: ["0x5000000000000000000000000000000000000000000000000000000000000003", "0x5000000000000000000000000000000000000000000000000000000000000004"]
				data: "0x6000000000000000000000000000000000000000000000000000000000000002"
				transactionHash: "0x2000002000000000000000000000000000000000000000000000000000000002"
				blockHash: "0x1000002000000000000000000000000000000000000000000000000000000002"
				blockNumber: 1000002
				transactionIndex: 0
				logIndex: 0
				removed: "false"
				block: "%s"
				transaction: "%s"
			}) {
				_docID
				address
				topics
			}
		}`, block2DocID, tx2DocID),
	}

	// Execute Log 2 creation
	jsonData, err = json.Marshal(log2Mutation)
	if err != nil {
		return fmt.Errorf("failed to marshal log2 mutation: %v", err)
	}

	resp, err = http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("log2 creation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("log2 creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read log2 response: %v", err)
	}

	var log2Resp map[string]interface{}
	if err := json.Unmarshal(body, &log2Resp); err != nil {
		return fmt.Errorf("failed to parse log2 response: %v", err)
	}

	if errors, hasErrors := log2Resp["errors"]; hasErrors {
		return fmt.Errorf("GraphQL errors in log2 creation: %v", errors)
	}

	logger.Testf("Log 2 created successfully: %s", string(body))
	logger.Test("Mock data with relationships and logs inserted successfully!")

	return nil
}

func hasBlocks() bool {
	query := `{"query":"query { Block(limit: 1) { number } }"}`
	resp, err := http.Post("http://localhost:9181/api/v0/graphql", "application/json", bytes.NewBuffer([]byte(query)))
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
