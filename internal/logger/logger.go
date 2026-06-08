package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new zap logger based on configuration
func New(level, format string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	var encoder zapcore.EncoderConfig
	if format == "console" {
		encoder = zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
	} else {
		encoder = zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
	}

	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	var encoderCore zapcore.Core
	if format == "console" {
		encoderCore = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoder),
			zapcore.AddSync(os.Stdout),
			lvl,
		)
	} else {
		encoderCore = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoder),
			zapcore.AddSync(os.Stdout),
			lvl,
		)
	}

	logger := zap.New(encoderCore, opts...)
	return logger, nil
}

// Sync flushes any buffered log entries
func Sync(logger *zap.Logger) {
	if logger != nil {
		_ = logger.Sync()
	}
}
