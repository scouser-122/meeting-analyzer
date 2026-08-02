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

// NewSummaryService creates new SummaryService instance
func NewSummaryService(
	summaryRepo repository.SummaryRepository,
	repositoryUtils repository.RepositoryUtils,
) *SummaryService {
	service := SummaryService{}
	service.summaryRepo = summaryRepo
	service.repositoryUtils = repositoryUtils
	return &service
}

func (s *SummaryService) AddNewSummary(ctx context.Context, summary *model.Summary) error {
	return s.summaryRepo.Create(ctx, summary)
}
