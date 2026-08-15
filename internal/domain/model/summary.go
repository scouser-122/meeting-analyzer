package model

import "time"

// Summary — краткая выжимка (саммари) текста встречи, полученная от GigaChat.
type Summary struct {
	ID        string
	MeetingID string
	UserID    string
	Text      string
	CreatedAt time.Time
	SearchV   TSVector
}
