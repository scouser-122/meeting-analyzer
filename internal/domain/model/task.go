package model

import "time"

// TaskStatus — статус задачи обработки встречи.
type TaskStatus string

const (
	TaskStatusCreated     TaskStatus = "created"
	TaskStatusProcessing  TaskStatus = "processing"
	TaskStatusTranscribed TaskStatus = "transcribed"
	TaskStatusSummarized  TaskStatus = "summarized"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusFailed      TaskStatus = "failed"
)

// Task — задача асинхронной обработки встречи.
type Task struct {
	ID           string
	MeetingID    string
	Status       TaskStatus
	ErrorMessage *string // nil если нет ошибки
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
