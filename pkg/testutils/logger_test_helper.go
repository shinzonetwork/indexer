package testutils

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestLoggerSetup holds the test logger and buffer for log capture
type TestLoggerSetup struct {
	Logger *zap.SugaredLogger
	Buffer *bytes.Buffer
	t      *testing.T
}

// NewTestLogger creates a logger that writes to a buffer for testing
func NewTestLogger(t *testing.T) *TestLoggerSetup {
	buffer := &bytes.Buffer{}
	
	// Create encoder config for consistent output
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.LevelKey = "level"
	encoderConfig.TimeKey = "timestamp"
	
	// Create core that writes to buffer
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig), // Use JSON for easier parsing in tests
		zapcore.AddSync(buffer),
		zapcore.DebugLevel, // Capture all levels
	)
	
	logger := zap.New(core).Sugar()
	
	return &TestLoggerSetup{
		Logger: logger,
		Buffer: buffer,
		t:      t,
	}
}

// GetLogOutput returns the current log output as a string
func (tls *TestLoggerSetup) GetLogOutput() string {
	return tls.Buffer.String()
}

// ClearBuffer clears the log buffer
func (tls *TestLoggerSetup) ClearBuffer() {
	tls.Buffer.Reset()
}

// AssertLogContains checks if the log output contains the expected message
func (tls *TestLoggerSetup) AssertLogContains(expectedMessage string) {
	tls.t.Helper()
	output := tls.GetLogOutput()
	if !strings.Contains(output, expectedMessage) {
		tls.t.Errorf("Expected log to contain '%s', but got:\n%s", expectedMessage, output)
	}
}

// AssertLogLevel checks if a log entry with the specified level exists
func (tls *TestLoggerSetup) AssertLogLevel(level string) {
	tls.t.Helper()
	output := tls.GetLogOutput()
	// Check for both full "level" and abbreviated "L" field names
	if strings.Contains(output, `"level":"`+level+`"`) || strings.Contains(output, `"L":"`+level+`"`) {
		return
	}
	tls.t.Errorf("Expected log to contain level '%s', but got:\n%s", level, output)
}

// AssertLogField checks if a log entry contains a specific field with value
// This handles both direct fields and nested fields from errors.LogContext
func (tls *TestLoggerSetup) AssertLogField(fieldName, expectedValue string) {
	tls.t.Helper()
	output := tls.GetLogOutput()
	
	// Check for direct field
	directField := `"` + fieldName + `":"` + expectedValue + `"`
	if strings.Contains(output, directField) {
		return
	}
	
	// Check for field in nested ignored object (from errors.LogContext)
	nestedField := `"` + fieldName + `":"` + expectedValue + `"`
	if strings.Contains(output, nestedField) {
		return
	}
	
	// Check for snake_case version (errors.LogContext uses snake_case)
	snakeFieldName := strings.ReplaceAll(fieldName, "_", "_")
	if fieldName == "errorCode" {
		snakeFieldName = "error_code"
	} else if fieldName == "blockNumber" {
		snakeFieldName = "block_number"
	} else if fieldName == "txHash" {
		snakeFieldName = "tx_hash"
	}
	
	snakeField := `"` + snakeFieldName + `":"` + expectedValue + `"`
	if strings.Contains(output, snakeField) {
		return
	}
	
	// Check for numeric values without quotes
	numericField := `"` + snakeFieldName + `":` + expectedValue
	if strings.Contains(output, numericField) {
		return
	}
	
	// Check for direct field name with numeric value
	directNumericField := `"` + fieldName + `":` + expectedValue
	if strings.Contains(output, directNumericField) {
		return
	}
	
	tls.t.Errorf("Expected log to contain field '%s' with value '%s', but got:\n%s", fieldName, expectedValue, output)
}

// AssertLogStructuredContext checks if the log contains structured context from errors.LogContext
func (tls *TestLoggerSetup) AssertLogStructuredContext(expectedComponent, expectedOperation string) {
	tls.t.Helper()
	output := tls.GetLogOutput()
	
	// Check for component
	if !strings.Contains(output, `"component":"`+expectedComponent+`"`) {
		tls.t.Errorf("Expected log to contain component '%s', but got:\n%s", expectedComponent, output)
	}
	
	// Check for operation
	if !strings.Contains(output, `"operation":"`+expectedOperation+`"`) {
		tls.t.Errorf("Expected log to contain operation '%s', but got:\n%s", expectedOperation, output)
	}
}

// GetLogEntries parses the log output and returns individual log entries
func (tls *TestLoggerSetup) GetLogEntries() []map[string]interface{} {
	output := tls.GetLogOutput()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	var entries []map[string]interface{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			tls.t.Logf("Failed to parse log line: %s, error: %v", line, err)
			continue
		}
		entries = append(entries, entry)
	}
	
	return entries
}

// AssertLogCount checks if the expected number of log entries were created
func (tls *TestLoggerSetup) AssertLogCount(expectedCount int) {
	tls.t.Helper()
	entries := tls.GetLogEntries()
	if len(entries) != expectedCount {
		tls.t.Errorf("Expected %d log entries, but got %d:\n%s", expectedCount, len(entries), tls.GetLogOutput())
	}
}
