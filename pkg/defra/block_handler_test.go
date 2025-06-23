package defra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"shinzo/version1/pkg/types"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNewBlockHandler(t *testing.T) {
	host := "localhost"
	port := 9181

	handler := NewBlockHandler(host, port)

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
	// Create a test logger
	logger := zap.NewNop().Sugar()
	handler := NewBlockHandler("localhost", 9181)

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
			result := handler.ConvertHexToInt(tt.input, logger)
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

	logger := zap.NewNop().Sugar()

	block := &types.Block{
		Hash:         "0x1234567890abcdef",
		Number:       "12345",
		Timestamp:    "1600000000",
		ParentHash:   "0xabcdef1234567890",
		Difficulty:   "1000000",
		GasUsed:      "4000000",
		GasLimit:     "8000000",
		Nonce:        "123456789",
		Miner:        "0xminer",
		Size:         "1024",
		StateRoot:    "0xstateroot",
		Sha3Uncles:   "0xsha3uncles",
		ReceiptsRoot: "0xreceiptsroot",
		ExtraData:    "extra",
	}

	docID := handler.CreateBlock(context.Background(), block, logger)

	if docID != "test-block-doc-id" {
		t.Errorf("Expected docID 'test-block-doc-id', got '%s'", docID)
	}
}

func TestCreateTransaction_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"create_Transaction": {
					"_docID": "test-tx-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

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
		Nonce:            "1",
		TransactionIndex: "0",
		Status:           true,
	}

	blockID := "test-block-id"
	docID := handler.CreateTransaction(context.Background(), tx, blockID, logger)

	if docID != "test-tx-doc-id" {
		t.Errorf("Expected docID 'test-tx-doc-id', got '%s'", docID)
	}
}

func TestUpdateTransactionRelationships_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"update_Transaction": {
					"_docID": "updated-tx-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	blockID := "test-block-id"
	txHash := "0xtxhash"

	docID := handler.UpdateTransactionRelationships(context.Background(), blockID, txHash, logger)

	if docID != "updated-tx-doc-id" {
		t.Errorf("Expected docID 'updated-tx-doc-id', got '%s'", docID)
	}
}

func TestGetHighestBlockNumber_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"Block": [
					{
						"number": "12345"
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	blockNumber := handler.GetHighestBlockNumber(context.Background(), logger)

	if blockNumber != 12345 {
		t.Errorf("Expected block number 12345, got %d", blockNumber)
	}
}

func TestGetHighestBlockNumber_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"Block": []
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	blockNumber := handler.GetHighestBlockNumber(context.Background(), logger)

	if blockNumber != 0 {
		t.Errorf("Expected block number 0 for empty response, got %d", blockNumber)
	}
}

func TestPostToCollection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		response := `{
			"data": {
				"create_TestCollection": {
					"_docID": "test-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	data := map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	}

	docID := handler.PostToCollection(context.Background(), "TestCollection", data, logger)

	if docID != "test-doc-id" {
		t.Errorf("Expected docID 'test-doc-id', got '%s'", docID)
	}
}

func TestPostToCollection_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	data := map[string]interface{}{
		"field1": "value1",
	}

	docID := handler.PostToCollection(context.Background(), "TestCollection", data, logger)

	// Should return empty string on error
	if docID != "" {
		t.Errorf("Expected empty docID on error, got '%s'", docID)
	}
}

func TestSendToGraphql_Success(t *testing.T) {
	expectedQuery := "query { test }"
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request body to verify the query
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedQuery = string(body)

		response := `{"data": {"test": "result"}}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	request := types.Request{
		Query: expectedQuery,
	}

	result := handler.SendToGraphql(context.Background(), request, logger)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Verify the query was sent correctly
	if !strings.Contains(receivedQuery, expectedQuery) {
		t.Errorf("Request body should contain query '%s', got '%s'", expectedQuery, receivedQuery)
	}
}

func TestCreateLog_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"create_Log": {
					"_docID": "test-log-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	log := &types.Log{
		Address:          "0xcontract",
		Topics:           []string{"0xtopic1", "0xtopic2"},
		Data:             "0xlogdata",
		BlockNumber:      "12345",
		TransactionHash:  "0xtxhash",
		TransactionIndex: "0",
		BlockHash:        "0xblockhash",
		LogIndex:         "0",
		Removed:          false,
	}

	blockID := "test-block-id"
	txID := "test-tx-id"

	docID := handler.CreateLog(context.Background(), log, blockID, txID, logger)

	if docID != "test-log-doc-id" {
		t.Errorf("Expected docID 'test-log-doc-id', got '%s'", docID)
	}
}

func TestCreateEvent_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": {
				"create_Event": {
					"_docID": "test-event-doc-id"
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	handler := &BlockHandler{
		defraURL: server.URL,
		client:   &http.Client{},
	}

	logger := zap.NewNop().Sugar()

	event := &types.Event{
		ContractAddress:  "0xcontract",
		EventName:        "Transfer",
		Parameters:       "0xeventdata",
		TransactionHash:  "0xtxhash",
		BlockHash:        "0xblockhash",
		BlockNumber:      "12345",
		TransactionIndex: "0",
		LogIndex:         "0",
	}

	logID := "test-log-id"

	docID := handler.CreateEvent(context.Background(), event, logID, logger)

	if docID != "test-event-doc-id" {
		t.Errorf("Expected docID 'test-event-doc-id', got '%s'", docID)
	}
}
