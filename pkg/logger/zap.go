package logger

import (
	"go.uber.org/zap"
)

var Sugar *zap.SugaredLogger

func Init(development bool) {

	var zapLevel zap.AtomicLevel
	if development {
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	config := zap.Config{
		Level:            zapLevel,
		Development:      development,
		Encoding:         "json",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout", "logfile"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	Sugar = logger.Sugar()
}
