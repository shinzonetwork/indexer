package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Sugar *zap.SugaredLogger

// using the logger looks like this:

// logger.Sugar.Info("here is a log example");
// or
// logger := logger.Sugar()
// logger.Info("here is a log example")

func Init(development bool) {
	var zapLevel zapcore.Level
	if development {
		zapLevel = zap.DebugLevel
	} else {
		zapLevel = zap.InfoLevel
	}

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Create console writer (stdout)
	consoleWriter := zapcore.Lock(os.Stdout)
	
	// Try to create logs directory and file writers
	logsDir := "../../logs"
	var cores []zapcore.Core
	
	if err := os.MkdirAll(logsDir, 0755); err == nil {
		// Directory exists or was created successfully
		logFile := filepath.Join(logsDir, "logfile.log")
		errorFile := filepath.Join(logsDir, "errorfile.log")
		
		// Create file writer for all logs
		if logFileWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			// Core for console output
			consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, zapLevel)
			cores = append(cores, consoleCore)
			
			// Core for all logs to logfile
			logFileCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.AddSync(logFileWriter), zapLevel)
			cores = append(cores, logFileCore)
			
			// Create file writer for errors only
			if errorFileWriter, err := os.OpenFile(errorFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				// Core for ERROR level logs only to errorfile
				errorCore := zapcore.NewCore(
					zapcore.NewConsoleEncoder(encoderConfig),
					zapcore.AddSync(errorFileWriter),
					zapcore.ErrorLevel, // Only ERROR level and above
				)
				cores = append(cores, errorCore)
			}
		} else {
			// Fallback to console only if file creation fails
			consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, zapLevel)
			cores = append(cores, consoleCore)
		}
	} else {
		// Fallback to console only if directory creation fails
		consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, zapLevel)
		cores = append(cores, consoleCore)
	}
	
	// Combine all cores
	core := zapcore.NewTee(cores...)
	logger := zap.New(core)
	
	Sugar = logger.Sugar()
}
