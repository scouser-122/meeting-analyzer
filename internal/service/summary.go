package service

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
)

// SummaryService service to work with transcription summary
type SummaryService struct {
	summaryRepo     repository.SummaryRepository
	repositoryUtils repository.RepositoryUtils
}

// NewSummaryService creates new SummaryService instance.
func NewSummaryService(
	summaryRepo repository.SummaryRepository,
	repositoryUtils repository.RepositoryUtils,
) *SummaryService {
	service := SummaryService{}
	service.summaryRepo = summaryRepo
	service.repositoryUtils = repositoryUtils
	return &service
}

// AddNewSummary persists a new transcription summary.
func (s *SummaryService) AddNewSummary(ctx context.Context, summary *model.Summary) error {
	return s.summaryRepo.Create(ctx, summary)
}

// GetSummary returns the summary text for a meeting, or nil if not found.
func (s *SummaryService) GetSummary(ctx context.Context, meetingID string) (*string, error) {
	summary, err := s.summaryRepo.GetByMeetingID(ctx, meetingID)
	if err != nil {
		if err.Error() == "summary not found" {
			return nil, nil
		}
		return nil, err
	}
	return &summary.Text, nil
}

// FindByTextContains searches summaries by text using full-text search.
func (s *SummaryService) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Summary, error) {
	return s.summaryRepo.FindByTextContains(ctx, userID, textPart)
}
