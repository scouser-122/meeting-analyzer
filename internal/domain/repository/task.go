package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// TaskRepository — интерфейс доступа к данным задач обработки.
type TaskRepository interface {
	Create(ctx context.Context, meetingID string) (*model.Task, error)
	GetByID(ctx context.Context, id string) (*model.Task, error)
	GetByMeetingID(ctx context.Context, meetingID string) (*model.Task, error)
	UpdateStatus(ctx context.Context, id string, status model.TaskStatus, errorMessage *string) error
	DeleteByMeetingID(ctx context.Context, meetingID string) error
}
