package models

// QueryIntent — структурированный результат разбора вопроса
type QueryIntent struct {
	IsMeetingQuery bool     `json:"is_meeting_query"`
	Keywords       []string `json:"keywords"`
	Topic          string   `json:"topic"`
}
