package model

import "time"

// Transcription — текстовая расшифровка аудиозаписи встречи.
type Transcription struct {
	ID        string
	MeetingID string
	UserID    string
	Text      string
	CreatedAt time.Time
	SearchV   TSVector
}
