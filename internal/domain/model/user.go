package model

import "time"

// User — пользователь системы.
type User struct {
	ID         string
	ExternalID string // идентификатор из мессенджера / TUI
	CreatedAt  time.Time
}
