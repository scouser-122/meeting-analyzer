package repository

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// TranscriptionRepository — интерфейс доступа к данным транскрипций.
type TranscriptionRepository interface {
	Create(ctx context.Context, transcription *model.Transcription) error
	GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error)
}
