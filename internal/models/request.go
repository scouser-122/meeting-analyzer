package models

// FindMeetingRequest represents a request body for searching meetings by keywords.
type FindMeetingRequest struct {
	UserID   string `json:"user_id"`
	KeyWords string `json:"key_words"`
}

// ChatRequest represents a request body for the chat assistant endpoint.
type ChatRequest struct {
	UserID   string `json:"user_id"`
	Question string `json:"question"`
}

// RetryMeetingRequest represents a request body for retrying meeting processing.
type RetryMeetingRequest struct {
	UserID    string `json:"user_id"`
	MeetingID string `json:"meeting_id"`
}
