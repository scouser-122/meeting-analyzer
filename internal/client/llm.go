package client

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// LLMClient defines the contract for large language model clients.
type LLMClient interface {
	SummarizeTranscription(ctx context.Context, meeting *model.Meeting, transcriptionText string) (string, error)
	ExtractIntent(ctx context.Context, text string) (*models.QueryIntent, error)
}
