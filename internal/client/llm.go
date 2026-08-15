package client

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

type LLMClient interface {
	SummarizeTranscription(ctx context.Context, meeting *model.Meeting, transcriptionText string) (string, error)
	ExtractIntent(ctx context.Context, text string) (*models.QueryIntent, error)
}
