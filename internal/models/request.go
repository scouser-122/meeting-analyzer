package models

type FindMeetingRequest struct {
	UserID   string `json:"user_id"`
	KeyWords string `json:"key_words"`
}

type ChatRequest struct {
	UserID   string `json:"user_id"`
	Question string `json:"question"`
}
