package defra

import (
	"context"
	"net/http"
	"os"
	"shinzo/version1/pkg/errors"
	"shinzo/version1/pkg/logger"
	"shinzo/version1/pkg/testutils"
	"shinzo/version1/pkg/types"
	"shinzo/version1/pkg/utils"
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
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	// Test NewBlockHandler
	host := "localhost"
	port := 9181

	handler, err := NewBlockHandler(host, port)
	if err != nil {
		// Instead of just t.Errorf, create a structured error and log it
		handlerErr := errors.NewConfigurationError(
			"defra",
			"NewBlockHandler",
			"failed to create handler",
			"host=localhost, port=9181",
			err,
			errors.WithMetadata("host", host),
			errors.WithMetadata("port", port),
		)

		// Log with structured context
		logCtx := errors.LogContext(handlerErr)
		testLogger.Logger.With(logCtx).Error("Handler creation failed")

		// Verify the structured logging worked
		testLogger.AssertLogLevel("ERROR")
		testLogger.AssertLogContains("Handler creation failed")
		testLogger.AssertLogStructuredContext("defra", "NewBlockHandler")
		testLogger.AssertLogField("host", "localhost")
		testLogger.AssertLogField("port", "9181")
		testLogger.AssertLogField("errorCode", "CONFIGURATION_ERROR")

		// Still fail the test
		testLogger.Logger.Errorf("Expected no error, got '%v'", err)
		return
	}

	if handler == nil {
		// Create a structured error for nil handler
		nilErr := errors.NewConfigurationError(
			"defra",
			"NewBlockHandler",
			"handler is nil after successful creation",
			"host=localhost, port=9181",
			nil,
			errors.WithMetadata("host", host),
			errors.WithMetadata("port", port),
		)

		logCtx := errors.LogContext(nilErr)
		testLogger.Logger.With(logCtx).Error("NewBlockHandler returned nil")

		// Verify logging
		testLogger.AssertLogLevel("ERROR")
		testLogger.AssertLogStructuredContext("defra", "NewBlockHandler")
		testLogger.AssertLogField("errorCode", "CONFIGURATION_ERROR")

		testLogger.Logger.Fatal("NewBlockHandler should not return nil")
	}

	// Log successful creation
	testLogger.Logger.With(
		"component", "defra",
		"operation", "NewBlockHandler",
		"host", host,
		"port", port,
		"defraURL", handler.defraURL,
	).Info("Block handler created successfully")

	expectedURL := "http://localhost:9181/api/v0/graphql"
	if handler.defraURL != expectedURL {
		// Create structured error for URL mismatch
		urlErr := errors.NewConfigurationError(
			"defra",
			"NewBlockHandler",
			"incorrect defraURL configured",
			"expected="+expectedURL+", actual="+handler.defraURL,
			nil,
			errors.WithMetadata("expected", expectedURL),
			errors.WithMetadata("actual", handler.defraURL),
		)

		logCtx := errors.LogContext(urlErr)
		testLogger.Logger.With(logCtx).Error("DefraURL mismatch", "expected", expectedURL, "actual", handler.defraURL)

		// Verify logging
		testLogger.AssertLogLevel("ERROR")
		testLogger.AssertLogStructuredContext("defra", "NewBlockHandler")
		testLogger.AssertLogField("expected", expectedURL)
		testLogger.AssertLogField("actual", handler.defraURL)

		testLogger.Logger.Errorf("Expected defraURL %s, got %s", expectedURL, handler.defraURL)
	}

	if handler.client == nil {
		// Create structured error for nil client
		clientErr := errors.NewConfigurationError(
			"defra",
			"NewBlockHandler",
			"HTTP client is nil",
			"handler created but client not initialized",
			nil,
		)

		logCtx := errors.LogContext(clientErr)
		testLogger.Logger.With(logCtx).Error("HTTP client is nil")

		// Verify logging
		testLogger.AssertLogLevel("ERROR")
		testLogger.AssertLogStructuredContext("defra", "NewBlockHandler")
		testLogger.AssertLogField("errorCode", "CONFIGURATION_ERROR")

		testLogger.Logger.Error("HTTP client should not be nil")
	}

	// Verify we logged success
	testLogger.Logger.Info("Block handler created successfully", "component", "defra", "operation", "NewBlockHandler")
}

func TestConvertHexToInt(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)

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
				logCtx := errors.LogContext(err)
				testLogger.Logger.With(logCtx).Error("ConvertHexToInt failed")
			}
			if result != tt.expected {
				logCtx := errors.LogContext(err)
				testLogger.Logger.With(logCtx).Error("ConvertHexToInt failed")
			}
		})
	}
}

func TestCreateBlock_MockServer(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "test-block-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestConvertHexToInt_UnhappyPaths(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
				logCtx := errors.LogContext(err)
				testLogger.Logger.With(logCtx).Error("ConvertHexToInt failed")
			}
			if result != 0 {
				logCtx := errors.LogContext(err)
				testLogger.Logger.With(logCtx).Error("ConvertHexToInt failed")
			}
		})
	}
}

func TestCreateBlock_InvalidBlock(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateBlock_InvalidJSON(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateBlock_MissingField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateBlock_EmptyField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {"create_Block": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	block := &types.Block{Hash: "0x1", Number: "1"}
	result, err := handler.CreateBlock(context.Background(), block)
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateTransaction_MockServer(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
	if docID != "test-tx-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateTransaction_InvalidBlockNumber(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateLog_MockServer(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "test-log-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestCreateLog_InvalidBlockNumber(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateTransactionRelationships_MockServerSuccess(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := testutils.CreateGraphQLUpdateResponse("Transaction", "updated-tx-doc-id")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockID := "test-block-id"
	txHash := "0xtxhash"

	docID, err := handler.UpdateTransactionRelationships(context.Background(), blockID, txHash)
	if err != nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "updated-tx-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateTransactionRelationships_InvalidJSON(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateTransactionRelationships_MissingField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateTransactionRelationships_EmptyField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {"update_Transaction": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateTransactionRelationships_NilResponse(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	result, err := handler.UpdateTransactionRelationships(context.Background(), "blockId", "txHash")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateLogRelationships_MockServerSuccess(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {"update_Log": [{"_docID": "log-doc-id"}]}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err != nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "log-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateLogRelationships_InvalidJSON(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := "not a json"
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateLogRelationships_MissingField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateLogRelationships_EmptyField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {"update_Log": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateLogRelationships_NilResponse(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	result, err := handler.UpdateLogRelationships(context.Background(), "blockId", "txId", "txHash", "logIndex")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestUpdateEventRelationships_EmptyField(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := `{"data": {"update_Event": []}}`
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	result, err := handler.UpdateEventRelationships(context.Background(), "logDocId", "txHash", "logIndex")
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestPostToCollection_Success(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	config := testutils.MockServerConfig{
		ResponseBody: testutils.CreateGraphQLCreateResponse("TestCollection", "test-doc-id"),
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			if r.Method != "POST" {
				return errors.NewHTTPConnectionFailed(
					"defra",
					"PostToCollection",
					"POST request expected",
					nil,
					errors.WithMetadata("method", r.Method),
				)
			}
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				return errors.NewHTTPConnectionFailed(
					"defra",
					"PostToCollection",
					"POST request expected",
					nil,
					errors.WithMetadata("contentType", contentType),
				)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if docID != "test-doc-id" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestPostToCollection_ServerError(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if docID != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestPostToCollection_NilResponse(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close() // Simulate network error, SendToGraphql returns nil

	data := map[string]interface{}{
		"field1": "value1",
	}
	result, err := handler.PostToCollection(context.Background(), "TestCollection", data)
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if result != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
	// Note: We don't test log output since we're using global logger
}

func TestSendToGraphql_Success(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	expectedQuery := "query { test }"

	config := testutils.MockServerConfig{
		ResponseBody: `{"data": {"test": "result"}}`,
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
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
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestSendToGraphql_NetworkError(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	// Create a server and close it before making the request
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close()

	request := types.Request{Query: "query { test }", Type: "POST"}
	result, err := handler.SendToGraphql(context.Background(), request)
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if result != nil && string(result) != "" {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestGetHighestBlockNumber_MockServer(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := testutils.CreateGraphQLQueryResponse("Block", `[
		{
			"number": 12345
		}
	]`)
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockNumber, err := handler.GetHighestBlockNumber(context.Background())
	if err != nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	if blockNumber != 12345 {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}

func TestGetHighestBlockNumber_EmptyResponse(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	response := testutils.CreateGraphQLQueryResponse("Block", "[]")
	server, handler := createBlockHandlerWithMocks(response)
	defer server.Close()

	blockNumber, err := handler.GetHighestBlockNumber(context.Background())
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if blockNumber != 0 {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

}

func TestGetHighestBlockNumber_NilResponse(t *testing.T) {
	// Set up test logger
	testLogger := testutils.NewTestLogger(t)
	server, handler := createBlockHandlerWithMocks(`{"data": {}}`)
	server.Close() // Simulate network error, SendToGraphql returns nil

	result, err := handler.GetHighestBlockNumber(context.Background())
	if err == nil {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}

	// Should return 0 even when error occurs
	if result != 0 {
		logCtx := errors.LogContext(err)
		testLogger.Logger.With(logCtx).Error("Block creation failed")
	}
}
