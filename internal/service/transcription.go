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

// NewTranscriptionService creates new TranscriptionService instance.
func NewTranscriptionService(
	transcriptionsRepo repository.TranscriptionRepository,
	repositoryUtils repository.RepositoryUtils,
) *TranscriptionService {
	service := TranscriptionService{}
	service.transcriptionsRepo = transcriptionsRepo
	service.repositoryUtils = repositoryUtils
	return &service
}

// AddNewTranscription persists a new transcription record.
func (t *TranscriptionService) AddNewTranscription(ctx context.Context, transcription *model.Transcription) error {
	return t.transcriptionsRepo.Create(ctx, transcription)
}

// GetByMeetingID returns the transcription for the specified meeting.
func (t *TranscriptionService) GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error) {
	transcription, err := t.transcriptionsRepo.GetByMeetingID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	return transcription, nil
}

// FindByTextContains searches transcriptions by text using full-text search.
func (t *TranscriptionService) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Transcription, error) {
	return t.transcriptionsRepo.FindByTextContains(ctx, userID, textPart)
}

// FindByKeyWords searches transcriptions by keywords and topic using full-text search.
func (t *TranscriptionService) FindByKeyWords(ctx context.Context, userID string, keywords []string, topic string) ([]*model.Transcription, error) {
	return t.transcriptionsRepo.FindByKeyWords(ctx, userID, keywords, topic)
}
