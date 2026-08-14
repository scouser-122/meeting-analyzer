package model

import "time"

// User — пользователь системы.
type User struct {
	ID        string `json:"id"`
	CreatedAt time.Time
}
