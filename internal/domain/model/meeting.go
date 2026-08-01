package model

import "time"

// Meeting — встреча (аудиозапись), загруженная пользователем.
type Meeting struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Name             *string   `json:"name"`
	FilePath         *string   `json:"-"` // путь к загруженному аудиофайлу
	OriginalFilename *string   `json:"-"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`
}
