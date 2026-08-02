package processors

import "github.com/scouser-122/meeting-analyzer/internal/domain/model"

type SummarizeProcessor interface {
	SummarizeTranscription(meeting *model.Meeting, text string) (string, error)
}
