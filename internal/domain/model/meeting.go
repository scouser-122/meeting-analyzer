package model

import "time"

// Meeting — встреча (аудиозапись), загруженная пользователем.
type Meeting struct {
	ID               string
	UserID           string
	FilePath         string // путь к загруженному аудиофайлу
	OriginalFilename string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
