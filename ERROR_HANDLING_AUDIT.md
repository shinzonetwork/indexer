# Error Handling Migration Guide

## Overview
This document provides specific recommendations for migrating from the current `FatalError` system to the new comprehensive `IndexerError` system designed for distributed blockchain indexing.

## New Error System Summary

### Error Types Available:
1. **NetworkError** - RPC/HTTP communication issues (often retryable)
2. **DataError** - Parsing/validation failures (usually non-retryable)  
3. **StorageError** - Database operation failures (sometimes retryable)
4. **SystemError** - Critical system-level failures (requires attention)

### Key Features:
- **Severity Levels**: Info, Warning, Error, Critical
- **Retry Behavior**: NonRetryable, Retryable, RetryableWithBackoff
- **Structured Context**: Component, operation, block number, tx hash, metadata
- **Error Codes**: Standardized codes for monitoring/metrics
- **Error Wrapping**: Maintains underlying error chains

---

## File-by-File Migration Plan

### 🔧 **1. pkg/defra/block_handler.go**

**Current Issues:**
- Uses custom `FatalError` interface with non-standard `Error() (string, string)` signature
- Manual error construction with hardcoded line numbers
- No error categorization or retry guidance

**Required Changes:**

**Line 22-34**: Replace FatalError interface and implementation
```go
// REMOVE: Current FatalError interface and FatalErrorImpl
// REPLACE WITH: Import new error system
import "shinzo/version1/pkg/errors"
```

**Line 38-46**: Update NewBlockHandler function
```go
// CURRENT:
func NewBlockHandler(host string, port int) (*BlockHandler, FatalError) {
	if host == "" {
		return nil, &FatalErrorImpl{
			message: "host is empty",
			log:     "NewBlockHandler: host:string is empty; failed at line 38",
		}
	}
	// ... etc

// REPLACE WITH:
func NewBlockHandler(host string, port int) (*BlockHandler, error) {
	if host == "" {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandler", 
			"host parameter is empty", nil)
	}
	if port == 0 {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandler", 
			"port parameter is zero", nil)
	}
	return &BlockHandler{
		defraURL: fmt.Sprintf("http://%s:%d/api/v0/graphql", host, port),
		client:   &http.Client{},
	}, nil
}
```

**Line 54-76**: Update ConvertHexToInt function
```go
// CURRENT:
func (h *BlockHandler) ConvertHexToInt(s string) (int64, FatalError) {
	if s == "" {
		return 0, &FatalErrorImpl{
			message: "Empty hex string provided",
			log:     "ConvertHexToInt: s:string is empty; failed at line 59",
		}
	}
	// ... parsing logic
	if err != nil {
		return 0, &FatalErrorImpl{
			message: err.Error(),
			log:     "ConvertHexToInt: failed to parse hex string; failed at line 73",
		}
	}

// REPLACE WITH:
func (h *BlockHandler) ConvertHexToInt(s string) (int64, error) {
	if s == "" {
		return 0, errors.NewInvalidHex("defra", "ConvertHexToInt", s, nil)
	}
	
	// Remove "0x" prefix if present
	hexStr := s
	if strings.HasPrefix(s, "0x") {
		hexStr = s[2:]
	}

	// Parse the hex string
	blockInt, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return 0, errors.NewInvalidHex("defra", "ConvertHexToInt", s, err)
	}

	return blockInt, nil
}
```

**Line 85+**: Update all Create* functions to use StorageError for DB operations
```go
// Example for CreateBlock:
func (h *BlockHandler) CreateBlock(ctx context.Context, block *types.Block) (string, error) {
	// Input validation - use DataError
	if block == nil {
		return "", errors.NewInvalidBlockFormat("defra", "CreateBlock", nil)
	}
	
	// Data conversion - use DataError
	blockInt, err := h.ConvertHexToInt(block.Number)
	if err != nil {
		return "", err // Already properly wrapped
	}
	
	// Database operation - use StorageError
	docID := h.PostToCollection(ctx, "Block", blockData)
	if docID == "" {
		return "", errors.NewQueryFailed("defra", "CreateBlock", nil)
	}
	
	return docID, nil
}
```

### 🌐 **2. pkg/rpc/ethereum_client.go**

**Current Issues:**
- Network errors not categorized properly
- No retry guidance for RPC failures
- Missing context information for debugging

**Required Changes:**

**Lines with RPC calls**: Wrap network operations
```go
// CURRENT: Direct RPC calls without proper error handling
client, err := ethclient.Dial(nodeURL)
if err != nil {
    return nil, err
}

// REPLACE WITH:
client, err := ethclient.Dial(nodeURL)
if err != nil {
    return nil, errors.NewRPCConnectionFailed("rpc", "NewEthereumClient", err)
}

// For timeout scenarios:
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
block, err := client.BlockByNumber(ctx, blockNumber)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return nil, errors.NewRPCTimeout("rpc", "BlockByNumber", err, 
            errors.WithBlockNumber(blockNumber.Int64()))
    }
    return nil, errors.NewRPCConnectionFailed("rpc", "BlockByNumber", err,
        errors.WithBlockNumber(blockNumber.Int64()))
}
```

### 📊 **3. pkg/types/conversion.go**

**Current Issues:**
- Data parsing errors not properly categorized
- No context about what data failed to parse

**Required Changes:**

**Line 86**: Update block number conversion
```go
// CURRENT:
BlockNumber: strconv.Itoa(blockNumber),

// REPLACE WITH: Add error handling
blockNumberStr := strconv.Itoa(blockNumber)
if blockNumber < 0 {
    return nil, errors.NewParsingFailed("types", "ConvertReceipt", 
        "block number", nil, errors.WithBlockNumber(int64(blockNumber)))
}
```

### 🚀 **4. cmd/block_poster/main.go**

**Current Issues:**
- No retry logic based on error types
- All errors treated the same way
- Missing structured error logging

**Required Changes:**

**Line 36-40**: Update error handling for NewBlockHandler
```go
// CURRENT:
blockHandler, fatalErr := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)
if fatalErr != nil {
    msg, log := fatalErr.Error()
    logger.Sugar.Fatalf("Failed to create block handler: %s - %s", msg, log)
}

// REPLACE WITH:
blockHandler, err := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)
if err != nil {
    // Log with structured context
    logCtx := errors.LogContext(err)
    logger.Sugar.With(logCtx).Fatalf("Failed to create block handler: %v", err)
}
```

**Line 70+**: Implement smart retry logic in main processing loop
```go
// CURRENT: Simple sleep on any error
if err != nil {
    logger.Sugar.Error("Failed to create block in DefraDB: ", err)
    time.Sleep(time.Second * 3)
    continue
}

// REPLACE WITH: Smart retry based on error type
blockDocId, err := blockHandler.CreateBlock(context.Background(), block)
if err != nil {
    logCtx := errors.LogContext(err)
    logger.Sugar.With(logCtx).Error("Failed to create block in DefraDB")
    
    // Check if error is retryable
    if errors.IsRetryable(err) {
        retryDelay := errors.GetRetryDelay(err, retryAttempts)
        logger.Sugar.Warnf("Retrying block creation after %v", retryDelay)
        time.Sleep(retryDelay)
        retryAttempts++
        continue
    }
    
    // Non-retryable error - skip this block and continue with next
    if errors.IsDataError(err) {
        logger.Sugar.Errorf("Skipping block %d due to data error: %v", blockNum, err)
        continue
    }
    
    // Critical error - may need to exit
    if errors.IsCritical(err) {
        logger.Sugar.Fatalf("Critical error processing block %d: %v", blockNum, err)
    }
}
```

### 📝 **5. pkg/logger/zap.go** 

**Enhancement Needed:**
Add helper function for structured error logging:

```go
// ADD: New helper function for error logging
func LogError(err error, message string, fields ...zap.Field) {
    if indexerErr, ok := err.(errors.IndexerError); ok {
        ctx := indexerErr.Context()
        allFields := []zap.Field{
            zap.String("error_code", indexerErr.Code()),
            zap.String("severity", indexerErr.Severity().String()),
            zap.String("retryable", indexerErr.Retryable().String()),
            zap.String("component", ctx.Component),
            zap.String("operation", ctx.Operation),
            zap.Time("error_timestamp", ctx.Timestamp),
            zap.Error(err),
        }
        
        if ctx.BlockNumber != nil {
            allFields = append(allFields, zap.Int64("block_number", *ctx.BlockNumber))
        }
        
        if ctx.TxHash != nil {
            allFields = append(allFields, zap.String("tx_hash", *ctx.TxHash))
        }
        
        // Add custom fields
        allFields = append(allFields, fields...)
        
        // Log at appropriate level based on severity
        switch indexerErr.Severity() {
        case errors.Critical:
            Sugar.Errorw(message, allFields...)
        case errors.Error:
            Sugar.Errorw(message, allFields...)
        case errors.Warning:
            Sugar.Warnw(message, allFields...)
        case errors.Info:
            Sugar.Infow(message, allFields...)
        }
    } else {
        Sugar.With(fields...).Errorw(message, "error", err)
    }
}
```

---

## Implementation Priority

### Phase 1 (Critical - Do First):
1. ✅ **pkg/errors/** - New error system (COMPLETED)
2. 🔧 **pkg/defra/block_handler.go** - Core database operations
3. 🚀 **cmd/block_poster/main.go** - Main processing loop with retry logic

### Phase 2 (Important):
4. 🌐 **pkg/rpc/ethereum_client.go** - Network error handling  
5. 📊 **pkg/types/conversion.go** - Data parsing errors

### Phase 3 (Enhancement):
6. 📝 **pkg/logger/zap.go** - Structured logging helpers
7. 🧪 **All test files** - Update test expectations for new error types

---

## Testing the Migration

After implementing changes:

```bash
# Test error package
go build ./pkg/errors

# Test individual packages
go build ./pkg/defra
go build ./pkg/rpc
go build ./pkg/types

# Test main application
go build ./cmd/block_poster

# Run all tests
go test ./... -v
```

---

## Key Benefits After Migration

1. **Operational Excellence**: Clear retry guidance for different error types
2. **Observability**: Structured error logging with context and codes
3. **Debugging**: Rich context (block numbers, tx hashes, metadata)
4. **Monitoring**: Standardized error codes for metrics and alerting
5. **Reliability**: Smart retry logic based on error characteristics
6. **Maintainability**: Consistent error handling patterns across codebase

The new system transforms basic error handling into production-ready distributed systems error management that can handle the complexity and scale requirements of blockchain data ingestion.
