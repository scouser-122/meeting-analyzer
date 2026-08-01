package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
)

// RepositoryUtils declares interface with repository utils
type RepositoryUtils interface {
	// CreateTransaction creates transaction to be used in several operations
	CreateTransaction(ctx context.Context) (models.GenericTransaction, error)

	// CommitTransaction commits transaction changes to DB
	CommitTransaction(ctx context.Context, tx models.GenericTransaction) error
}
