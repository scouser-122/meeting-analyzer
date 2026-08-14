package client

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

type AudioProcessor interface {
	TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error)
}
