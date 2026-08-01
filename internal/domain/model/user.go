package model

import "time"

// User — пользователь системы.
type User struct {
	ID         string  `json:"id"`
	ExternalID *string `json:"external_id"` // идентификатор из мессенджера / TUI
	CreatedAt  time.Time
}
