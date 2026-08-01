package model

import "time"

// Meeting — встреча (аудиозапись), загруженная пользователем.
type Meeting struct {
	ID               string  `json:"id"`
	UserID           string  `json:"user_id"`
	FilePath         *string // путь к загруженному аудиофайлу
	OriginalFilename *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
