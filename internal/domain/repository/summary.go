package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// SummaryRepository — интерфейс доступа к данным кратких выжимок.
type SummaryRepository interface {
	Create(ctx context.Context, summary *model.Summary) error
	GetByMeetingID(ctx context.Context, meetingID string) (*model.Summary, error)
	FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Summary, error)
}
