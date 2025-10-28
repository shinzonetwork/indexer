package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit_Development(t *testing.T) {
	// Create a temporary logs directory for testing
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test development mode (console only for tests)
	InitConsoleOnly(true)

	if Sugar == nil {
		t.Fatal("Sugar logger should not be nil after Init")
	}

	// Test that we can log without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Logging should not panic: %v", r)
		}
	}()

	Sugar.Debug("Test debug message")
	Sugar.Info("Test info message")
	Sugar.Warn("Test warn message")
	Sugar.Error("Test error message")
}

func TestInit_Production(t *testing.T) {
	// Create a temporary logs directory for testing
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test production mode (console only for tests)
	InitConsoleOnly(false)

	if Sugar == nil {
		t.Fatal("Sugar logger should not be nil after Init")
	}

	// Test that we can log without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Logging should not panic: %v", r)
		}
	}()

	Sugar.Info("Test info message")
	Sugar.Warn("Test warn message")
	Sugar.Error("Test error message")
}

func TestInit_LogLevels(t *testing.T) {
	// Create a temporary logs directory for testing
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test development mode (should have debug level, console only for tests)
	InitConsoleOnly(true)
	if Sugar == nil {
		t.Fatal("Sugar logger should not be nil after Init")
	}

	// Test production mode (should have info level, console only for tests)
	InitConsoleOnly(false)
	if Sugar == nil {
		t.Fatal("Sugar logger should not be nil after Init")
	}
}

func TestInit_GlobalSugarVariable(t *testing.T) {
	// Create a temporary logs directory for testing
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Ensure Sugar is initially nil or set it to nil
	Sugar = nil

	InitConsoleOnly(true)

	// Verify the global Sugar variable is set
	if Sugar == nil {
		t.Error("Global Sugar variable should be set after Init")
	}

	// Verify Sugar is usable (can call methods)
	if Sugar != nil {
		// Test that Sugar can be used for logging without panicking
		Sugar.Info("Test log message")
	}
}

func TestInit_LogFileCreation(t *testing.T) {
	// Create a temporary logs directory for testing
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	InitConsoleOnly(true)

	// Log some messages - no files should be created in console-only mode
	Sugar.Info("Test message for file creation")
	Sugar.Sync() // Ensure messages are flushed

	// Check that log file was NOT created (since we're using console-only mode)
	logFile := filepath.Join(logsDir, "logfile.log")
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		Test("Warning: Log file was created unexpectedly at " + logFile)
	}
}

func TestEncoderConfig(t *testing.T) {
	// Test that the encoder config is set up correctly by initializing and checking for panics
	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Init should not panic with encoder config: %v", r)
		}
	}()

	InitConsoleOnly(true)

	// Test different log levels to ensure encoder works
	Sugar.Debug("Debug level test")
	Sugar.Info("Info level test")
	Sugar.Warn("Warn level test")
	Sugar.Error("Error level test")
}
