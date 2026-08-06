package models

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// CommonResponse defines common response structure with status and message
type CommonResponse struct {
	// Status response status
	Status string `json:"status"`
	// Message response message
	Message string `json:"message,omitempty"`
	// Data response data
	Data any `json:"data,omitempty"`
}

// NewErrorResponseBuffer creates byte buffer for error response
// to be passed in Write method of ResponseWriter
func NewErrorResponseBuffer(message string) []byte {
	result := CommonResponse{
		Status:  "error",
		Message: message,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(result); err != nil {
		return []byte{}
	}
	return buf.Bytes()
}

func WriteResponseError(err *CustomErr, res http.ResponseWriter) {
	res.Header().Set("Content-Type", "application/json")
	if err.HTTPStatus != 0 {
		res.WriteHeader(err.HTTPStatus)
	}
	res.Write(NewErrorResponseBuffer(err.Message))
}

// NewSuccessResponseBuffer creates byte buffer for success response
// to be passed in Write method of ResponseWriter
func NewSuccessResponseBuffer(message string) []byte {
	return NewSuccessResponseBufferWithData(message, nil)
}

// NewSuccessResponseBufferWithData creates byte buffer for success response
// to be passed in Write method of ResponseWriter
func NewSuccessResponseBufferWithData(message string, data any) []byte {

	result := CommonResponse{
		Status:  "ok",
		Message: message,
		Data:    data,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(result); err != nil {
		return []byte{}
	}
	return buf.Bytes()
}

type MeetingResponseData struct {
	ID                  string    `json:"id"`
	Name                *string   `json:"name"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
	Status              string    `json:"status"`
	Summary             *string   `json:"summary,omitempty"`
	ProcessErrorMessage *string   `json:"process_error_message,omitempty"`
}

type TranscriptionResponseData struct {
	Text string `json:"text"`
}

type ChatResponse struct {
	Answer string `json:"answer"`
}
