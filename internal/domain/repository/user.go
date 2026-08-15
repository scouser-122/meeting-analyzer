package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// UserRepository — интерфейс доступа к данным пользователей.
type UserRepository interface {
	Create(ctx context.Context, id string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}
