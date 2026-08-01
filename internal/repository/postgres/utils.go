package postgres

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// PostgresRepositoryUtils implements RepositoryUtils interface to work with Postgres DB
type PostgresRepositoryUtils struct {
	Database *PostgresDatabase
}

// NewPostgresRepositoryUtils creates new PostgresRepositoryUtils instance
func NewPostgresRepositoryUtils(database *PostgresDatabase) *PostgresRepositoryUtils {
	return &PostgresRepositoryUtils{
		Database: database,
	}
}

// CreateTransaction creates transaction to be used in several methods
func (u *PostgresRepositoryUtils) CreateTransaction(ctx context.Context) (models.GenericTransaction, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	tx, err := u.Database.Begin(ctx)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return tx, nil
}

// CommitTransaction commits transaction changes to DB
func (u *PostgresRepositoryUtils) CommitTransaction(ctx context.Context, tx models.GenericTransaction) error {
	return DataBaseRequestRetry(
		ctx,
		u.Database.Config.RetryConfig,
		func() error {
			return tx.Commit(ctx)
		},
	)
}
