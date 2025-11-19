# Testing with Structured Error Logging

This guide shows how to use structured error logging in your tests with buffer capture to verify log output.

## Overview

The testing framework provides:
1. **TestLoggerSetup** - A test helper that captures logs in a buffer
2. **Custom Error Types** - Structured errors with context from `pkg/errors`
3. **Assertion Helpers** - Functions to verify log content and structure

## Quick Start

### 1. Set up a test logger

```go
func TestMyFunction(t *testing.T) {
    testLogger := testutils.NewTestLogger(t)
    
    // Your test code here...
}
```

### 2. Create structured errors

```go
// Network error example
networkErr := errors.NewRPCTimeout(
    "defra",                    // component
    "CreateBlock",              // operation
    "block data",               // input data
    originalErr,                // underlying error
    errors.WithBlockNumber(12345),
    errors.WithMetadata("endpoint", "http://localhost:8545"),
)

// Data error example
dataErr := errors.NewInvalidHex(
    "defra",
    "ConvertTransaction",
    "0xInvalidHex",
    originalErr,
    errors.WithTxHash("0x123abc"),
)
```

### 3. Log with structured context

```go
// Log the error using the same pattern as main.go
logCtx := errors.LogContext(networkErr)
testLogger.Logger.With(logCtx).Error("Failed to create block in DefraDB")
```

### 4. Assert on log output

```go
// Check log level
testLogger.AssertLogLevel("ERROR")

// Check log message
testLogger.AssertLogContains("Failed to create block in DefraDB")

// Check structured context
testLogger.AssertLogStructuredContext("defra", "CreateBlock")

// Check specific fields
testLogger.AssertLogField("blockNumber", "12345")
testLogger.AssertLogField("endpoint", "http://localhost:8545")
testLogger.AssertLogField("errorCode", "RPC_TIMEOUT")
testLogger.AssertLogField("severity", "ERROR")
testLogger.AssertLogField("retryable", "RETRYABLE")
```

## Available Error Types

### Network Errors (Usually Retryable)
- `NewRPCTimeout` - RPC request timeouts
- `NewRPCConnectionFailed` - Connection failures
- `NewRateLimited` - Rate limiting errors

### Data Errors (Usually Non-Retryable)
- `NewInvalidHex` - Invalid hexadecimal strings
- `NewInvalidBlockFormat` - Malformed block data
- `NewParsingFailed` - General parsing failures

### Storage Errors (Sometimes Retryable)
- `NewDBConnectionFailed` - Database connection issues
- `NewQueryFailed` - Database query failures
- `NewDocumentNotFound` - Missing documents

### System Errors (Critical)
- `NewConfigurationError` - Configuration issues
- `NewServiceUnavailable` - Service unavailability

## Error Context Options

Add context to errors using these options:

```go
errors.WithBlockNumber(12345)
errors.WithTxHash("0x123abc")
errors.WithMetadata("key", "value")
```

## Helper Functions

### TestLoggerSetup Methods

- `GetLogOutput()` - Get raw log output
- `ClearBuffer()` - Clear the log buffer
- `AssertLogContains(message)` - Check if log contains message
- `AssertLogLevel(level)` - Check for specific log level
- `AssertLogField(field, value)` - Check for field with value
- `AssertLogStructuredContext(component, operation)` - Check error context
- `GetLogEntries()` - Parse log entries as JSON objects
- `AssertLogCount(count)` - Check number of log entries

## Error Type Checking

Test error types and retry behavior:

```go
// Check error type
if errors.IsNetworkError(err) {
    testLogger.Logger.Info("Confirmed network error")
}

// Check retry behavior
if errors.IsRetryable(err) {
    testLogger.Logger.Warn("Error is retryable")
} else {
    testLogger.Logger.Info("Error is not retryable")
}

// Check severity
if errors.IsCritical(err) {
    testLogger.Logger.Error("Critical error detected")
}
```

## Complete Example

```go
func TestBlockCreationWithErrorLogging(t *testing.T) {
    testLogger := testutils.NewTestLogger(t)
    
    // Create a retryable network error
    networkErr := errors.NewRPCTimeout(
        "defra",
        "CreateBlock",
        "block data",
        fmt.Errorf("request timeout"),
        errors.WithBlockNumber(12345),
    )
    
    // Log the error with structured context
    logCtx := errors.LogContext(networkErr)
    testLogger.Logger.With(logCtx).Error("Block creation failed")
    
    // Verify the log output
    testLogger.AssertLogLevel("ERROR")
    testLogger.AssertLogContains("Block creation failed")
    testLogger.AssertLogStructuredContext("defra", "CreateBlock")
    testLogger.AssertLogField("blockNumber", "12345")
    testLogger.AssertLogField("errorCode", "RPC_TIMEOUT")
    testLogger.AssertLogField("retryable", "RETRYABLE")
    
    // Test retry logic
    if errors.IsRetryable(networkErr) {
        testLogger.Logger.Warn("Retrying operation")
        testLogger.AssertLogContains("Retrying operation")
    }
}
```

## Integration with Existing Tests

To refactor existing tests:

1. Replace `t.Errorf` with structured error creation and logging
2. Add buffer assertions to verify log output
3. Use the same error types and logging patterns as your main application
4. Keep the original test assertions (`t.Errorf`, `t.Fatal`) for test failures

This ensures your tests use the same error handling patterns as your production code while providing detailed verification of log output.
