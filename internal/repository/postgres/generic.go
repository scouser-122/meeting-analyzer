package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scouser-122/meeting-analyzer/internal/config"
)

// QueryExecutor interface for DB queries
type QueryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// GenericRepository generic Postgres repository to work with any entity
type GenericRepository[T any] struct {
	db          QueryExecutor
	retryConfig config.RetryConfig
	table       string
	keyName     string
	mapper      func(row pgx.Row) (*T, error)
}

// NewGenericRepository creates generic repository
func NewGenericRepository[T any](
	db *PostgresDatabase,
	table string,
	keyName string,
	mapper func(row pgx.Row) (*T, error),
) *GenericRepository[T] {
	return &GenericRepository[T]{
		db:          db,
		retryConfig: db.Config.RetryConfig,
		table:       table,
		keyName:     keyName,
		mapper:      mapper,
	}
}
