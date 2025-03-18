package views

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*ViewManager, *httptest.Server) {
	server := httptest.NewServer(handler)
	cfg := &Config{
		Host: server.URL[7:], // Remove "http://"
		Port: 0,             // Port is included in the URL
	}
	vm := NewViewManager(cfg)
	vm.defraURL = server.URL + "/graphql"
	// Use a shorter timeout for tests
	vm.client.Timeout = 1 * time.Second
	return vm, server
}

func TestQueryBlocks(t *testing.T) {
	mockBlocks := []map[string]interface{}{
		{
			"hash":       "0x123",
			"number":     "1",
			"time":      "2025-03-17T18:36:41-04:00",
			"gasUsed":    "21000",
			"difficulty": "2",
		},
	}

	vm, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		// Return mock response immediately
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"blocks": mockBlocks,
			},
		})
	})
	defer server.Close()

	ctx := context.Background()
	opts := &QueryOptions{
		Pagination: &PaginationOptions{
			Limit:  10,
			Offset: 0,
		},
		Fields:     []string{"hash", "number", "time", "gasUsed", "difficulty"},
		Timeout:    500 * time.Millisecond,
		MaxRetries: 1,
		RetryDelay: 100 * time.Millisecond,
	}

	blocks, err := vm.QueryBlocks(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, mockBlocks, blocks)

	// Check metrics
	metrics := vm.GetMetrics()
	assert.Equal(t, int64(1), metrics["total_queries"])
	assert.Equal(t, int64(0), metrics["error_count"])
	assert.Equal(t, int64(1), metrics["successful_views"].(map[string]int64)["QueryBlocks"])
}

func TestRetryMechanism(t *testing.T) {
	attempts := 0
	vm, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"blocks": []map[string]interface{}{},
			},
		})
	})
	defer server.Close()

	ctx := context.Background()
	opts := &QueryOptions{
		MaxRetries: 3,
		RetryDelay: 50 * time.Millisecond,
		Timeout:    200 * time.Millisecond,
	}

	_, err := vm.QueryBlocks(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)

	// Check metrics
	metrics := vm.GetMetrics()
	assert.Equal(t, int64(2), metrics["retry_count"])
}

func TestTimeout(t *testing.T) {
	vm, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"blocks": []map[string]interface{}{},
			},
		})
	})
	defer server.Close()

	ctx := context.Background()
	opts := &QueryOptions{
		Timeout:    100 * time.Millisecond,
		MaxRetries: 0, // Disable retries for this test
	}

	_, err := vm.QueryBlocks(ctx, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTimeout)

	// Check metrics
	metrics := vm.GetMetrics()
	assert.Equal(t, int64(1), metrics["error_count"])
}

func TestCreateView(t *testing.T) {
	view := &ViewConfig{
		ID:          "test-view",
		Name:        "Test View",
		Description: "A test view",
		Query:       "query { blocks { hash } }",
		Parameters:  map[string]string{"limit": "10"},
	}

	vm, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		variables := body["variables"].(map[string]interface{})
		input := variables["input"].(map[string]interface{})
		assert.Equal(t, view.ID, input["id"])
		assert.Equal(t, view.Name, input["name"])

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"createView": map[string]interface{}{
					"id":   view.ID,
					"name": view.Name,
				},
			},
		})
	})
	defer server.Close()

	err := vm.CreateView(context.Background(), view)
	require.NoError(t, err)

	// Check metrics
	metrics := vm.GetMetrics()
	assert.Equal(t, int64(1), metrics["successful_views"].(map[string]int64)["CreateView"])
}

func TestQueryEvents(t *testing.T) {
	mockEvents := []map[string]interface{}{
		{
			"index":           "0",
			"blockHash":       "0x123",
			"transactionHash": "0x456",
			"topics":         []string{"Transfer", "Approval"},
			"data":           "0x789",
		},
	}

	vm, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"events": mockEvents,
			},
		})
	})
	defer server.Close()

	ctx := context.Background()
	opts := &QueryOptions{
		Filter: map[string]interface{}{
			"blockHash": "0x123",
		},
		Timeout:    500 * time.Millisecond,
		MaxRetries: 1,
		RetryDelay: 100 * time.Millisecond,
	}

	events, err := vm.QueryEvents(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, mockEvents, events)

	// Check metrics
	metrics := vm.GetMetrics()
	assert.Equal(t, int64(1), metrics["successful_views"].(map[string]int64)["QueryEvents"])
}
