package defra

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"shinzo/version1/pkg/logger"
	"shinzo/version1/pkg/testutils"
	"shinzo/version1/pkg/types"
	"shinzo/version1/pkg/utils"
	"strings"
	"testing"

	"net/http/httptest"
)

// TestMain sets up testing environment
func TestMain(m *testing.M) {
	// Initialize logger for all tests
	logger.Init(true)

	// Run tests
	code := m.Run()

	// Exit with test result code
	os.Exit(code)
}

// createBlockHandlerWithMocksConfig creates a mock server and returns it along with a BlockHandler configured to use it, using a custom MockServerConfig.
func createBlockHandlerWithMocksConfig(config testutils.MockServerConfig) (*httptest.Server, *BlockHandler) {
	server := testutils.CreateMockServer(config)
	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}
	return server, handler
}

// createBlockHandlerWithMocks creates a mock server and returns it along with a BlockHandler configured to use it (simple version).
func createBlockHandlerWithMocks(response string) (*httptest.Server, *BlockHandler) {
	return createBlockHandlerWithMocksConfig(testutils.DefaultMockServerConfig(response))
}

func TestNewBlockHandler(t *testing.T) {
	host := "localhost"
	port := 9181

	handler, err := NewBlockHandler(host, port)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if handler == nil {
		t.Fatal("NewBlockHandler should not return nil")
	}

	expectedURL := "http://localhost:9181/api/v0/graphql"
	if handler.defraURL != expectedURL {
		t.Errorf("Expected defraURL %s, got %s", expectedURL, handler.defraURL)
	}

	if handler.client == nil {
		t.Error("HTTP client should not be nil")
	}
}

func TestConvertHexToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"Simple hex", "0x1", 1},
		{"Larger hex", "0xff", 255},
		{"Zero", "0x0", 0},
		{"Large number", "0x1000", 4096},
		{"Block number", "0x1234", 4660},
		{"All characters, lowercase", "0x1234567890abcdef", 1311768467294899695},
		{"All characters, uppercase", "0x1234567890ABCDEF", 1311768467294899695},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.HexToInt(tt.input)
			if err != nil {
				t.Errorf("ConvertHexToInt(%s) = %d, want %d", tt.input, result, tt.expected)
			}
			if result != tt.expected {
				t.Errorf("ConvertHexToInt(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateBlock_MockServer(t *testing.T) {
	// Create a mock DefraDB server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock successful block creation response
		response := `{
			"data": {
				"create_Block": {
					"_docID": "test-block-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create handler with test server URL
	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	block := &types.Block{
		Hash:         "0x1234567890abcdef",
		Number:       "12345",
		Timestamp:    "1600000000",
		ParentHash:   "0xabcdef1234567890",
		Difficulty:   "1000000",
		GasUsed:      "4000000",
		GasLimit:     "8000000",
		Nonce:        123456789,
		Miner:        "0xminer",
		Size:         "1024",
		StateRoot:    "0xstateroot",
		Sha3Uncles:   "0xsha3uncles",
		ReceiptsRoot: "0xreceiptsroot",
		ExtraData:    "extra",
	}

	docID, err := handler.CreateBlock(context.Background(), block)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if docID != "test-block-doc-id" {
		t.Errorf("Expected docID 'test-block-doc-id', got '%s'", docID)
	}
}

func TestConvertHexToInt_UnhappyPaths(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedLog string
	}{
		{"Empty string", "", "Empty hex string provided"},
		{"Invalid hex", "invalid hex", "Failed to parse hex string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.HexToInt(tt.input)
			if err == nil {
				t.Errorf("ConvertHexToInt(%s) = %d, want %d", tt.input, result, 0)
			}
			if result != 0 {
				t.Errorf("ConvertHexToInt(%s) = %d, want %d", tt.input, result, 0)
			}
		})
	}
}

func TestCreateBlock_InvalidBlock(t *testing.T) {
	response := testutils.CreateGraphQLCreateResponse("Block", "test-block-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{
		Hash:         "0x1234567890abcdef",
		Number:       "invalid block number",
		Timestamp:    "1600000000",
		ParentHash:   "0xabcdef1234567890",
		Difficulty:   "1000000",
		GasUsed:      "4000000",
		GasLimit:     "8000000",
		Nonce:        123456789,
		Miner:        "0xminer",
		Size:         "1024",
		StateRoot:    "0xstateroot",
		Sha3Uncles:   "0xsha3uncles",
		ReceiptsRoot: "0xreceiptsroot",
		ExtraData:    "extra",
	}

	docID, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if docID != "" {
		t.Error("Expected an error; should've received null response")
	}
}

func TestCreateBlock_InvalidJSON(t *testing.T) {
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if result != "" {
		t.Errorf("Expected empty string for invalid JSON, got '%s'", result)
	}
}

func TestCreateBlock_MissingField(t *testing.T) {
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if result != "" {
		t.Errorf("Expected empty string for missing field, got '%s'", result)
	}
}

func TestCreateBlock_EmptyField(t *testing.T) {
	response := `{"data": {"create_Block": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if result != "" {
		t.Errorf("Expected empty string for empty field, got '%s'", result)
	}
}

func TestCreateTransaction_MockServer(t *testing.T) {
	response := testutils.CreateGraphQLCreateResponse("Transaction", "test-tx-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	tx := &types.Transaction{
		Hash:             "0xtxhash",
		BlockHash:        "0xblockhash",
		BlockNumber:      "12345",
		From:             "0xfrom",
		To:               "0xto",
		Value:            "1000",
		Gas:              "21000",
		GasPrice:         "20000000000",
		Input:            "0xinput",
		Nonce:            1,
		TransactionIndex: 0,
		Status:           true,
	}

	blockID := "test-block-id"
	docID, err := handler.CreateTransaction(context.Background(), tx, blockID)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}
	if docID != "test-tx-doc-id" {
		t.Errorf("Expected docID 'test-tx-doc-id', got '%s'", docID)
	}
}

func TestCreateTransaction_InvalidBlockNumber(t *testing.T) {
	response := testutils.CreateGraphQLCreateResponse("Transaction", "test-tx-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	tx := &types.Transaction{
		Hash:             "0xtxhash",
		BlockHash:        "0xblockhash",
		BlockNumber:      "invalid block number",
		From:             "0xfrom",
		To:               "0xto",
		Value:            "1000",
		Gas:              "21000",
		GasPrice:         "20000000000",
		Input:            "0xinput",
		Nonce:            1,
		TransactionIndex: 0,
		Status:           true,
	}

	blockID := "test-block-id"
	docID, err := handler.CreateTransaction(context.Background(), tx, blockID)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if docID != "" {
		t.Error("Expected an error; should've received null response")
	}
}

func TestCreateLog_MockServer(t *testing.T) {
	response := testutils.CreateGraphQLCreateResponse("Log", "test-log-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	log := &types.Log{
		Address:          "0xcontract",
		Topics:           []string{"0xtopic1", "0xtopic2"},
		Data:             "0xlogdata",
		BlockNumber:      "12345",
		TransactionHash:  "0xtxhash",
		TransactionIndex: 0,
		BlockHash:        "0xblockhash",
		LogIndex:         0,
		Removed:          false,
	}

	blockID := "test-block-id"
	txID := "test-tx-id"

	docID, err := handler.CreateLog(context.Background(), log, blockID, txID)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if docID != "test-log-doc-id" {
		t.Errorf("Expected docID 'test-log-doc-id', got '%s'", docID)
	}
}

func TestCreateLog_InvalidBlockNumber(t *testing.T) {
	response := testutils.CreateGraphQLCreateResponse("Log", "test-log-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	logEntry := &types.Log{
		Address:          "0xcontract",
		Topics:           []string{"0xtopic1", "0xtopic2"},
		Data:             "0xlogdata",
		BlockNumber:      "invalid block number",
		TransactionHash:  "0xtxhash",
		TransactionIndex: 0,
		BlockHash:        "0xblockhash",
		LogIndex:         0,
		Removed:          false,
	}

	blockID := "test-block-id"
	txID := "test-tx-id"

	docID, err := handler.CreateLog(context.Background(), logEntry, blockID, txID)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if docID != "" {
		t.Error("Expected an error; should've received null response")
	}
}

func TestUpdateTransactionRelationships_MockServerSuccess(t *testing.T) {
	response := testutils.CreateGraphQLUpdateResponse("Transaction", "updated-tx-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockID := "test-block-id"
	txHash := "0xtxhash"

	docID, err := handler.UpdateTransactionRelationships(context.Background(), blockID, txHash)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if docID != "updated-tx-doc-id" {
		t.Errorf("Expected docID 'updated-tx-doc-id', got '%s'", docID)
	}
}

func TestUpdateTransactionRelationships_InvalidJSON(t *testing.T) {
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty string for invalid JSON, got '%s'", result)
	}
}

func TestUpdateTransactionRelationships_MissingField(t *testing.T) {
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty string for missing field, got '%s'", result)
	}
}

func TestUpdateTransactionRelationships_EmptyField(t *testing.T) {
	response := `{"data": {"update_Transaction": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty string for empty field, got '%s'", result)
	}
}

func TestUpdateTransactionRelationships_NilResponse(t *testing.T) {
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Error("Expected empty string for nil response")
	}
}

func TestUpdateLogRelationships_MockServerSuccess(t *testing.T) {
	response := `{"data": {"update_Log": [{"_docID": "log-doc-id"}]}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if result != "log-doc-id" {
		t.Errorf("Expected 'log-doc-id', got '%s'", result)
	}
}

func TestUpdateLogRelationships_InvalidJSON(t *testing.T) {
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Error("Expected empty string for invalid JSON")
	}
}

func TestUpdateLogRelationships_MissingField(t *testing.T) {
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != "" {
		t.Error("Expected empty string for missing field")
	}
}

func TestUpdateLogRelationships_EmptyField(t *testing.T) {
	response := `{"data": {"update_Log": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if result != "" {
		t.Error("Expected empty string for empty field")
	}
}

func TestUpdateLogRelationships_NilResponse(t *testing.T) {
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if result != "" {
		t.Error("Expected empty string for nil response")
	}
}

func TestUpdateEventRelationships_EmptyField(t *testing.T) {
	response := `{"data": {"update_Event": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateEventRelationships(context.Background(), "logDocId", "txHash", "logIndex")
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if result != "" {
		t.Error("Expected empty string for empty field")
	}
}

func TestPostToCollection_Success(t *testing.T) {
	config := testutils.MockServerConfig{
		ResponseBody: testutils.CreateGraphQLCreateResponse("TestCollection", "test-doc-id"),
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			if r.Method != "POST" {
				return fmt.Errorf("Expected POST request, got %s", r.Method)
			}
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				return fmt.Errorf("Expected Content-Type application/json, got %s", contentType)
			}
			return nil
		},
	}
	server, handler := createBlockHandlerWithMocksConfig(config)
	defer server.Close()

	data := map[string]interface{}{
		"string":      "value1",
		"number":      123,
		"bool":        true,
		"stringArray": []string{"dog", "cat", "bearded dragon"},
		"somethingElse": map[string]interface{}{
			"foo": "bar",
			"baz": 42,
		},
	}
	docID, err := handler.PostToCollection(context.Background(), "TestCollection", data)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if docID != "test-doc-id" {
		t.Errorf("Expected docID 'test-doc-id', got '%s'", docID)
	}
}

func TestPostToCollection_ServerError(t *testing.T) {
	server := testutils.CreateErrorServer(http.StatusInternalServerError, "Internal Server Error")
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	data := map[string]interface{}{
		"field1": "value1",
	}

	docID, err := handler.PostToCollection(context.Background(), "TestCollection", data)
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if docID != "" {
		t.Errorf("Expected empty docID on error, got '%s'", docID)
	}
}

func TestPostToCollection_NilResponse(t *testing.T) {
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close() // Simulate network error, SendToGraphql returns nil

	data := map[string]interface{}{
		"field1": "value1",
	}
	result, err := handler.PostToCollection(context.Background(), "TestCollection", data)
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if result != "" {
		t.Errorf("Expected empty docID on error, got '%s'", result)
	}
	// Note: We don't test log output since we're using global logger
}

func TestSendToGraphql_Success(t *testing.T) {
	expectedQuery := "query { test }"
	var receivedQuery string

	config := testutils.MockServerConfig{
		ResponseBody: `{"data": {"test": "result"}}`,
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
			receivedQuery = string(body)
			return nil
		},
	}
	server, handler := createBlockHandlerWithMocksConfig(config)
	defer server.Close()

	request := types.Request{
		Query: expectedQuery,
	}

	result, err := handler.SendToGraphql(context.Background(), request)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if !strings.Contains(receivedQuery, expectedQuery) {
		t.Errorf("Request body should contain query '%s', got '%s'", expectedQuery, receivedQuery)
	}
}

func TestSendToGraphql_NetworkError(t *testing.T) {
	// Create a server and close it before making the request
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	request := types.Request{Query: "query { test }", Type: "POST"}
	result, err := handler.SendToGraphql(context.Background(), request)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if result != nil && string(result) != "" {
		t.Errorf("Expected nil or empty result for network error, got '%s'", string(result))
	}
}

func TestGetHighestBlockNumber_MockServer(t *testing.T) {
	response := testutils.CreateGraphQLQueryResponse("Block", `[
		{
			"number": 12345
		}
	]`)
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockNumber, err := handler.GetHighestBlockNumber(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	if blockNumber != 12345 {
		t.Errorf("Expected block number 12345, got %d", blockNumber)
	}
}

func TestGetHighestBlockNumber_EmptyResponse(t *testing.T) {
	response := testutils.CreateGraphQLQueryResponse("Block", "[]")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockNumber, err := handler.GetHighestBlockNumber(context.Background())
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if blockNumber != 0 {
		t.Errorf("Expected block number 0 for empty response, got %d", blockNumber)
	}

}

func TestGetHighestBlockNumber_NilResponse(t *testing.T) {
	// Initialize logger for testing
	logger.Init(true)

	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close() // Simulate network error, SendToGraphql returns nil

	result, err := handler.GetHighestBlockNumber(context.Background())
	if err == nil {
		t.Error("Expected error when no blocks found, got nil")
	}

	// Should return 0 even when error occurs
	if result != 0 {
		t.Errorf("Expected block number 0 for empty response, got %d", result)
	}
	// Note: We don't test log output since we're using global logger
}
