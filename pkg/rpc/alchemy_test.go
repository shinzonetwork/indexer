package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"github.com/shinzonetwork/indexer/pkg/testutils"
	"strings"
	"testing"
	"time"
)

// createAlchemyClientWithMocks creates a mock server and returns it along with an AlchemyClient configured to use it.
func createAlchemyClientWithMocks(response string) (*httptest.Server, *AlchemyClient) {
	server := testutils.CreateMockServer(testutils.DefaultMockServerConfig(response))
	client := NewAlchemyClient("test-key")
	client.baseURL = server.URL
	return server, client
}

func TestNewAlchemyClient(t *testing.T) {
	apiKey := "test-api-key"
	client := NewAlchemyClient(apiKey)

	if client == nil {
		t.Fatal("NewAlchemyClient should not return nil")
	}

	if client.apiKey != apiKey {
		t.Errorf("Expected apiKey %s, got %s", apiKey, client.apiKey)
	}

	if client.baseURL != "https://eth-mainnet.alchemyapi.io/v2" {
		t.Errorf("Expected baseURL to be https://eth-mainnet.alchemyapi.io/v2, got %s", client.baseURL)
	}

	if client.client == nil {
		t.Error("HTTP client should not be nil")
	}
}

func TestAlchemyClient_GetBlock_Success(t *testing.T) {
	blockData := map[string]interface{}{
		"hash":         "0x1234567890abcdef",
		"number":       "0x1",
		"timestamp":    "0x5f5e100",
		"parentHash":   "0xabcdef1234567890",
		"transactions": []interface{}{},
	}
	response := testutils.CreateRPCNodeResponse(blockData)
	s, c := createAlchemyClientWithMocks(response)
	defer s.Close()

	ctx := context.Background()
	block, err := c.GetBlock(ctx, "0x1")

	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}

	if block == nil {
		t.Fatal("Block should not be nil")
	}

	if block.Hash != "0x1234567890abcdef" {
		t.Errorf("Expected hash 0x1234567890abcdef, got %s", block.Hash)
	}

	if block.Number != "0x1" {
		t.Errorf("Expected number 0x1, got %s", block.Number)
	}
}

func TestAlchemyClient_GetBlock_ServerError(t *testing.T) {
	s := testutils.CreateErrorServer(http.StatusInternalServerError, "Internal Server Error")
	defer s.Close()

	c := NewAlchemyClient("test-key")
	c.baseURL = s.URL

	ctx := context.Background()
	_, err := c.GetBlock(ctx, "0x1")

	if err == nil {
		t.Error("Expected error for server error, got nil")
	}
}

func TestAlchemyClient_GetTransactionReceipt_Success(t *testing.T) {
	receiptData := map[string]interface{}{
		"transactionHash": "0xabcdef1234567890",
		"blockHash":       "0x1234567890abcdef",
		"blockNumber":     "0x1",
		"transactionIndex": "0x0",
		"status":          "0x1",
		"gasUsed":         "0x5208",
		"logs":            []interface{}{},
	}
	response := testutils.CreateRPCNodeResponse(receiptData)
	s, c := createAlchemyClientWithMocks(response)
	defer s.Close()

	ctx := context.Background()
	receipt, err := c.GetTransactionReceipt(ctx, "0xabcdef1234567890")

	if err != nil {
		t.Fatalf("GetTransactionReceipt failed: %v", err)
	}

	if receipt == nil {
		t.Fatal("Receipt should not be nil")
	}

	if receipt.TransactionHash != "0xabcdef1234567890" {
		t.Errorf("Expected txHash 0xabcdef1234567890, got %s", receipt.TransactionHash)
	}

	if receipt.Status != "0x1" {
		t.Errorf("Expected status 0x1, got %s", receipt.Status)
	}
}

func TestAlchemyClient_GetTransactionReceipt_NotFound(t *testing.T) {
	response := testutils.CreateRPCNodeResponse(nil)
	s, c := createAlchemyClientWithMocks(response)
	defer s.Close()

	ctx := context.Background()
	receipt, err := c.GetTransactionReceipt(ctx, "0xnonexistent")

	if err != nil {
		t.Fatalf("GetTransactionReceipt failed: %v", err)
	}

	if receipt != nil {
		t.Error("Receipt should be nil for non-existent transaction")
	}
}

func TestAlchemyClient_Post_RequestFormat(t *testing.T) {
	// Create a test server that captures the request
	var capturedBody string
	var capturedHeaders http.Header

	config := testutils.MockServerConfig{
		ResponseBody: `{"jsonrpc": "2.0", "id": 1, "result": null}`,
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			capturedHeaders = r.Header
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
			capturedBody = string(body)
			return nil
		},
	}
	server := testutils.CreateMockServer(config)
	defer server.Close()

	client := NewAlchemyClient("test-key")
	client.baseURL = server.URL

	ctx := context.Background()
	payload := `{"jsonrpc": "2.0", "method": "test", "id": 1}`

	resp, err := client.post(ctx, payload)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify request format
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", capturedHeaders.Get("Content-Type"))
	}

	if !strings.Contains(capturedBody, payload) {
		t.Errorf("Request body should contain payload, got: %s", capturedBody)
	}
}

func TestAlchemyClient_Context_Cancellation(t *testing.T) {
	// Create a test server that delays response
	config := testutils.MockServerConfig{
		ResponseBody: `{"result": null}`,
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ValidateRequest: func(r *http.Request) error {
			time.Sleep(time.Second) // Delay to allow context cancellation
			return nil
		},
	}
	server := testutils.CreateMockServer(config)
	defer server.Close()

	client := NewAlchemyClient("test-key")
	client.baseURL = server.URL

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetBlock(ctx, "0x1")

	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}

	// Check that it's a context error
	if ctx.Err() == nil {
		t.Error("Context should have been cancelled")
	}
}
