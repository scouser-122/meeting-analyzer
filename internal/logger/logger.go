package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/alchemy/rotoslog"
	"github.com/hydraide/hydraide/app/server/loghandlers/slogmulti"
)

// Initialize creates and sets default slog logger with specified logging level
func Initialize(level string, fileHandler *rotoslog.Handler) {
	logLevel := slog.LevelInfo
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	}

	terminalHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	if fileHandler != nil {
		multiHandler := slogmulti.New(
			terminalHandler,
			fileHandler,
		)
		logger := slog.New(multiHandler)
		slog.SetDefault(logger)
	} else {
		logger := slog.New(terminalHandler)
		slog.SetDefault(logger)
	}
}

// LoggerKey key value to store logger in context
const LoggerKey string = "logger"

// GetSlogLoggerFromContext get logger from context
func GetSlogLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(LoggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
