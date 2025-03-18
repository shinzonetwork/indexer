package views

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Common errors
var (
	ErrInvalidResponse = errors.New("invalid response from DefraDB")
	ErrGraphQLError    = errors.New("GraphQL error")
	ErrTimeout        = errors.New("request timeout")
	ErrRetryExhausted = errors.New("retry attempts exhausted")
)

// ViewConfig represents a view configuration
type ViewConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Query       string            `json:"query"`
	Parameters  map[string]string `json:"parameters"`
}

// Config represents the configuration for the ViewManager
type Config struct {
	Host string
	Port int
}

// PaginationOptions provides configuration for paginated queries
type PaginationOptions struct {
	Limit  int `json:"limit"`  // Number of items per page
	Offset int `json:"offset"` // Starting position
}

// DefaultPaginationOptions returns default pagination settings
func DefaultPaginationOptions() *PaginationOptions {
	return &PaginationOptions{
		Limit:  100,
		Offset: 0,
	}
}

// QueryOptions provides configuration for GraphQL queries
type QueryOptions struct {
	Timeout    time.Duration
	Filter     map[string]interface{}
	Fields     []string
	Pagination *PaginationOptions
	MaxRetries int           // Maximum number of retry attempts
	RetryDelay time.Duration // Delay between retries
}

// DefaultQueryOptions returns the default query options
func DefaultQueryOptions() *QueryOptions {
	return &QueryOptions{
		Timeout:    30 * time.Second,
		Filter:     make(map[string]interface{}),
		Fields:     nil,
		Pagination: DefaultPaginationOptions(),
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// Metrics tracks query performance and errors
type Metrics struct {
	mu               sync.RWMutex
	queryCount       int64
	errorCount       int64
	totalLatency     time.Duration
	retryCount       int64
	lastError        error
	lastErrorTime    time.Time
	successfulViews  map[string]int64
	failedViews      map[string]int64
}

// ViewManager handles blockchain data views
type ViewManager struct {
	client   *http.Client
	defraURL string
	logger   *zap.Logger
	metrics  *Metrics
}

// NewViewManager creates a new ViewManager instance
func NewViewManager(cfg *Config) *ViewManager {
	return &ViewManager{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		defraURL: fmt.Sprintf("http://%s:%d/graphql", cfg.Host, cfg.Port),
		logger:   zap.NewExample(),
		metrics: &Metrics{
			successfulViews: make(map[string]int64),
			failedViews:    make(map[string]int64),
		},
	}
}

// recordMetrics updates query metrics
func (vm *ViewManager) recordMetrics(start time.Time, queryName string, err error) {
	vm.metrics.mu.Lock()
	defer vm.metrics.mu.Unlock()

	vm.metrics.queryCount++
	vm.metrics.totalLatency += time.Since(start)

	if err != nil {
		vm.metrics.errorCount++
		vm.metrics.lastError = err
		vm.metrics.lastErrorTime = time.Now()
		vm.metrics.failedViews[queryName]++
	} else {
		vm.metrics.successfulViews[queryName]++
	}
}

// GetMetrics returns current metrics
func (vm *ViewManager) GetMetrics() map[string]interface{} {
	vm.metrics.mu.RLock()
	defer vm.metrics.mu.RUnlock()

	avgLatency := time.Duration(0)
	if vm.metrics.queryCount > 0 {
		avgLatency = vm.metrics.totalLatency / time.Duration(vm.metrics.queryCount)
	}

	return map[string]interface{}{
		"total_queries":     vm.metrics.queryCount,
		"error_count":       vm.metrics.errorCount,
		"retry_count":       vm.metrics.retryCount,
		"avg_latency_ms":    avgLatency.Milliseconds(),
		"successful_views":  vm.metrics.successfulViews,
		"failed_views":      vm.metrics.failedViews,
		"last_error":        vm.metrics.lastError,
		"last_error_time":   vm.metrics.lastErrorTime,
	}
}

// executeQuery executes a GraphQL query with the given options
func (vm *ViewManager) executeQuery(ctx context.Context, query string, variables map[string]interface{}, opts *QueryOptions) ([]byte, error) {
	if opts == nil {
		opts = DefaultQueryOptions()
	}

	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			vm.metrics.mu.Lock()
			vm.metrics.retryCount++
			vm.metrics.mu.Unlock()
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(opts.RetryDelay):
			}
			
			vm.logger.Info("retrying query",
				zap.Int("attempt", attempt),
				zap.Error(lastErr),
			)
		}

		// Create context with timeout
		queryCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		defer cancel()

		// Add pagination to variables if configured
		if opts.Pagination != nil {
			variables["limit"] = opts.Pagination.Limit
			variables["offset"] = opts.Pagination.Offset
		}

		// Construct GraphQL query
		gqlQuery := map[string]interface{}{
			"query":     query,
			"variables": variables,
		}

		// Convert query to JSON
		jsonData, err := json.Marshal(gqlQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal query: %w", err)
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(queryCtx, "POST", vm.defraURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		resp, err := vm.client.Do(req)
		if err != nil {
			lastErr = err
			if queryCtx.Err() == context.DeadlineExceeded {
				lastErr = ErrTimeout
			}
			continue
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		return body, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
	}

	return nil, ErrRetryExhausted
}

// CreateView stores a new data view in DefraDB
func (vm *ViewManager) CreateView(ctx context.Context, view *ViewConfig) error {
	start := time.Now()
	defer vm.recordMetrics(start, "CreateView", nil)

	mutation := `
		mutation CreateView($input: ViewInput!) {
			createView(input: $input) {
				id
				name
			}
		}
	`

	variables := map[string]interface{}{
		"input": view,
	}

	body, err := vm.executeQuery(ctx, mutation, variables, nil)
	if err != nil {
		vm.recordMetrics(start, "CreateView", err)
		return err
	}

	var result struct {
		Data struct {
			CreateView struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"createView"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		vm.recordMetrics(start, "CreateView", err)
		return err
	}

	if len(result.Errors) > 0 {
		err = fmt.Errorf("%w: %s", ErrGraphQLError, result.Errors[0].Message)
		vm.recordMetrics(start, "CreateView", err)
		return err
	}

	return nil
}

// QueryBlocks retrieves blocks from DefraDB with pagination support
func (vm *ViewManager) QueryBlocks(ctx context.Context, opts *QueryOptions) ([]map[string]interface{}, error) {
	start := time.Now()
	defer vm.recordMetrics(start, "QueryBlocks", nil)

	if opts == nil {
		opts = DefaultQueryOptions()
	}

	// Default fields if none specified
	fields := opts.Fields
	if len(fields) == 0 {
		fields = []string{
			"hash",
			"number",
			"time",
			"parentHash",
			"difficulty",
			"gasUsed",
			"gasLimit",
			"nonce",
			"miner",
		}
	}

	// Build fields string
	fieldsStr := strings.Join(fields, "\n")

	query := fmt.Sprintf(`
		query GetBlocks($filter: BlockFilter, $limit: Int, $offset: Int) {
			blocks(filter: $filter, limit: $limit, offset: $offset) {
				%s
			}
		}
	`, fieldsStr)

	variables := map[string]interface{}{
		"filter": opts.Filter,
	}

	body, err := vm.executeQuery(ctx, query, variables, opts)
	if err != nil {
		vm.recordMetrics(start, "QueryBlocks", err)
		return nil, err
	}

	var result struct {
		Data struct {
			Blocks []map[string]interface{} `json:"blocks"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		vm.recordMetrics(start, "QueryBlocks", err)
		return nil, err
	}

	if len(result.Errors) > 0 {
		err = fmt.Errorf("%w: %s", ErrGraphQLError, result.Errors[0].Message)
		vm.recordMetrics(start, "QueryBlocks", err)
		return nil, err
	}

	return result.Data.Blocks, nil
}

// QueryTransactions retrieves transactions from DefraDB with pagination support
func (vm *ViewManager) QueryTransactions(ctx context.Context, opts *QueryOptions) ([]map[string]interface{}, error) {
	start := time.Now()
	defer vm.recordMetrics(start, "QueryTransactions", nil)

	if opts == nil {
		opts = DefaultQueryOptions()
	}

	// Default fields if none specified
	fields := opts.Fields
	if len(fields) == 0 {
		fields = []string{
			"hash",
			"from",
			"to",
			"contract",
			"value",
			"data",
			"gas",
			"gasPrice",
			"cost",
			"nonce",
			"state",
			"blockHash",
		}
	}

	// Build fields string
	fieldsStr := strings.Join(fields, "\n")

	query := fmt.Sprintf(`
		query GetTransactions($filter: TransactionFilter, $limit: Int, $offset: Int) {
			transactions(filter: $filter, limit: $limit, offset: $offset) {
				%s
			}
		}
	`, fieldsStr)

	variables := map[string]interface{}{
		"filter": opts.Filter,
	}

	body, err := vm.executeQuery(ctx, query, variables, opts)
	if err != nil {
		vm.recordMetrics(start, "QueryTransactions", err)
		return nil, err
	}

	var result struct {
		Data struct {
			Transactions []map[string]interface{} `json:"transactions"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		vm.recordMetrics(start, "QueryTransactions", err)
		return nil, err
	}

	if len(result.Errors) > 0 {
		err = fmt.Errorf("%w: %s", ErrGraphQLError, result.Errors[0].Message)
		vm.recordMetrics(start, "QueryTransactions", err)
		return nil, err
	}

	return result.Data.Transactions, nil
}

// QueryEvents retrieves events from DefraDB with pagination support
func (vm *ViewManager) QueryEvents(ctx context.Context, opts *QueryOptions) ([]map[string]interface{}, error) {
	start := time.Now()
	defer vm.recordMetrics(start, "QueryEvents", nil)

	if opts == nil {
		opts = DefaultQueryOptions()
	}

	// Default fields if none specified
	fields := opts.Fields
	if len(fields) == 0 {
		fields = []string{
			"index",
			"blockHash",
			"origin",
			"topics",
			"data",
			"transactionHash",
		}
	}

	// Build fields string
	fieldsStr := strings.Join(fields, "\n")

	query := fmt.Sprintf(`
		query GetEvents($filter: EventFilter, $limit: Int, $offset: Int) {
			events(filter: $filter, limit: $limit, offset: $offset) {
				%s
			}
		}
	`, fieldsStr)

	variables := map[string]interface{}{
		"filter": opts.Filter,
	}

	body, err := vm.executeQuery(ctx, query, variables, opts)
	if err != nil {
		vm.recordMetrics(start, "QueryEvents", err)
		return nil, err
	}

	var result struct {
		Data struct {
			Events []map[string]interface{} `json:"events"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		vm.recordMetrics(start, "QueryEvents", err)
		return nil, err
	}

	if len(result.Errors) > 0 {
		err = fmt.Errorf("%w: %s", ErrGraphQLError, result.Errors[0].Message)
		vm.recordMetrics(start, "QueryEvents", err)
		return nil, err
	}

	return result.Data.Events, nil
}
