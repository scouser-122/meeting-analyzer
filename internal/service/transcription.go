package service

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
)

// TranscriptionService service to work with transcriptions
type TranscriptionService struct {
	transcriptionsRepo repository.TranscriptionRepository
	repositoryUtils    repository.RepositoryUtils
}

// MeetingsService creates new MeetingsService instance
func NewTranscriptionService(
	transcriptionsRepo repository.TranscriptionRepository,
	repositoryUtils repository.RepositoryUtils,
) *TranscriptionService {
	service := TranscriptionService{}
	service.transcriptionsRepo = transcriptionsRepo
	service.repositoryUtils = repositoryUtils
	return &service
}

func (t *TranscriptionService) AddNewTranscription(ctx context.Context, transcription *model.Transcription) error {
	return t.transcriptionsRepo.Create(ctx, transcription)
}

func (t *TranscriptionService) GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error) {
	transcription, err := t.transcriptionsRepo.GetByMeetingID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	return transcription, nil
}

func (t *TranscriptionService) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Transcription, error) {
	return t.transcriptionsRepo.FindByTextContains(ctx, userID, textPart)
}
