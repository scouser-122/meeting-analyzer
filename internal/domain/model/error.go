package model

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorClassification defines error class for specific handling
type ErrorClassification int

const (
	// ErrorNonRetryable non retryable errors
	ErrorNonRetryable ErrorClassification = iota
	// ErrorRetryable errors
	ErrorRetryable
)

// ClassifyPostgreSQLError detects class of error which may happen during Postgres DB interaction
func ClassifyPostgreSQLError(err error) ErrorClassification {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.SerializationFailure,
			pgerrcode.DeadlockDetected,
			pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.InsufficientResources,
			pgerrcode.TooManyConnections,
			pgerrcode.LockNotAvailable:
			return ErrorRetryable
		}
	}
	var pgConnectErr *pgconn.ConnectError
	if errors.As(err, &pgConnectErr) {
		return ErrorRetryable
	}
	return ErrorNonRetryable
}
