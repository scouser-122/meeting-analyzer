package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// MeetingRepository — интерфейс доступа к данным встреч.
type MeetingRepository interface {
	Create(ctx context.Context, meeting *model.Meeting) error
	GetByID(ctx context.Context, id string) (*model.Meeting, error)
	GetByUserID(ctx context.Context, userID string) ([]model.Meeting, error)
	Update(ctx context.Context, meeting *model.Meeting) error
	Search(ctx context.Context, userID string, query string) ([]model.Meeting, error)
}
