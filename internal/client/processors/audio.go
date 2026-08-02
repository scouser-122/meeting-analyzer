package processors

import "github.com/scouser-122/meeting-analyzer/internal/domain/model"

type AudioProcessor interface {
	TranscribeAudio(meeting *model.Meeting) (string, error)
}
