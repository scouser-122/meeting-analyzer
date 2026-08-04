package model

import "time"

// User — пользователь системы.
type User struct {
	ID         string  `json:"id"`
	ExternalID *string `json:"external_id"`
	CreatedAt  time.Time
}
