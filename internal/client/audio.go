package client

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

// AudioProcessor defines the contract for audio transcription services.
type AudioProcessor interface {
	TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error)
}
