package client

import (
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

type LLMClient interface {
	SummarizeTranscription(meeting *model.Meeting, text string) (string, error)
	ExtractIntent(text string) (*models.QueryIntent, error)
}
