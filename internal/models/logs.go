package models

import "net/http"

// ResponseData to record response params for logger middleware
type ResponseData struct {
	Status int
	Size   int
}

// LoggingResponseWriter implements ResponseWriter interface for logging middleware
type LoggingResponseWriter struct {
	http.ResponseWriter
	ResponseData *ResponseData
}

// Write writes the data to the connection as part of an HTTP reply
func (r *LoggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.ResponseData.Size += size
	return size, err
}

// WriteHeader sends an HTTP response header with the provided
// status code.
func (r *LoggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.ResponseData.Status = statusCode
}
