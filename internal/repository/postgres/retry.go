package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// DataBaseRequestRetry implements retry mechanizm for DB interaction operations
func DataBaseRequestRetry(ctx context.Context, config config.RetryConfig, operation func() error) error {
	var lastErr error

	logger := logger.GetSlogLoggerFromContext(ctx)

	for attempt := 0; attempt <= config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if models.ClassifyPostgreSQLError(err) == models.ErrorNonRetryable {
			return lastErr
		}

		if attempt != config.MaxAttempts {
			backoff := calculateBackoff(config, attempt)
			logger.Info("received error on attempt", "attempt", attempt, "lastErr", lastErr, "backoff", backoff)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		} else {
			logger.Info("received error on attempt", "attempt", attempt, "lastErr", lastErr)
		}
	}

	logger.Info("max retries exceeded", "retries", config.MaxAttempts)
	return lastErr
}

func calculateBackoff(config config.RetryConfig, attempt int) time.Duration {
	backoff := float64(config.InitialBackoff + attempt*config.BackoffMultiplier)
	return time.Duration(backoff * float64(time.Millisecond))
}
