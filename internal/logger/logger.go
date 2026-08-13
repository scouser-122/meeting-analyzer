package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/alchemy/rotoslog"
	"github.com/hydraide/hydraide/app/server/loghandlers/slogmulti"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/models"
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
const LoggerKey models.ContextKey = "logger"

// GetSlogLoggerFromContext get logger from context
func GetSlogLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(LoggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

func StackFrames(err error) []string {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}
	var st stackTracer
	if !errors.As(err, &st) {
		return nil
	}
	frames := st.StackTrace()
	lines := make([]string, 0, len(frames))
	for _, f := range frames {
		lines = append(lines, fmt.Sprintf("%+v", f)) // "funcname\n\tfile:line" тоже экранируется, лучше по частям
	}
	return lines
}
