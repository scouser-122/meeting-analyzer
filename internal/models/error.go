package models

import (
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
)

// ErrorCode is a stable identifier for a custom application error.
// It should be used instead of comparing error messages or HTTP status codes.
type ErrorCode string

const (
	// ErrCodeUserNotFound is returned when a user does not exist.
	ErrCodeUserNotFound ErrorCode = "user_not_found"
	// ErrCodeUserAlreadyExists is returned when a user ID is already registered.
	ErrCodeUserAlreadyExists ErrorCode = "user_already_exists"
	// ErrCodeUserIDMissing is returned when a user ID is not provided.
	ErrCodeUserIDMissing ErrorCode = "user_id_missing"

	// ErrCodeMeetingNotFound is returned when a meeting does not exist.
	ErrCodeMeetingNotFound ErrorCode = "meeting_not_found"
	// ErrCodeMeetingIDMissing is returned when a meeting ID is not provided.
	ErrCodeMeetingIDMissing ErrorCode = "meeting_id_missing"
	// ErrCodeAccessDenied is returned when a user tries to access another user's data.
	ErrCodeAccessDenied ErrorCode = "access_denied"

	// ErrCodeTaskNotFound is returned when a processing task does not exist.
	ErrCodeTaskNotFound ErrorCode = "task_not_found"

	// ErrCodeTranscriptionNotFound is returned when a transcription does not exist.
	ErrCodeTranscriptionNotFound ErrorCode = "transcription_not_found"
	// ErrCodeTranscriptionFileEmpty is returned when an uploaded transcription file is empty.
	ErrCodeTranscriptionFileEmpty ErrorCode = "transcription_file_empty"

	// ErrCodeSummaryNotFound is returned when a summary does not exist.
	ErrCodeSummaryNotFound ErrorCode = "summary_not_found"

	// ErrCodeUnsupportedFileType is returned when an uploaded file has an unsupported extension.
	ErrCodeUnsupportedFileType ErrorCode = "unsupported_file_type"
	// ErrCodeFileTooLarge is returned when an uploaded file exceeds the size limit.
	ErrCodeFileTooLarge ErrorCode = "file_too_large"
	// ErrCodeFileUploadFailed is returned when a file could not be saved.
	ErrCodeFileUploadFailed ErrorCode = "file_upload_failed"
	// ErrCodeFileOpenFailed is returned when a file could not be opened from storage.
	ErrCodeFileOpenFailed ErrorCode = "file_open_failed"

	// ErrCodeInternal is returned for unexpected internal errors.
	ErrCodeInternal ErrorCode = "internal_error"

	// ErrCodeQueueFull is returned when the processing queue is full.
	ErrCodeQueueFull ErrorCode = "queue_full"
	// ErrCodeMeetingAlreadyProcessing is returned when a meeting is already being processed.
	ErrCodeMeetingAlreadyProcessing ErrorCode = "meeting_already_processing"
	// ErrCodeMeetingAlreadyCompleted is returned when a meeting has already been processed.
	ErrCodeMeetingAlreadyCompleted ErrorCode = "meeting_already_completed"
	// ErrCodeMeetingCannotRetry is returned when a meeting cannot be retried from its current status.
	ErrCodeMeetingCannotRetry ErrorCode = "meeting_cannot_retry"
)

// CustomErr specifies custom app error with code and message
type CustomErr struct {
	// Code is a stable identifier for the error.
	Code ErrorCode
	// Message error message
	Message string
	// HTTPStatus is status code which should be returned if such error happen
	HTTPStatus int
	// Cause is the original error that caused this custom error, if any.
	Cause error
}

// Error func to implement error interface
func (e *CustomErr) Error() string {
	return e.Message
}

// Unwrap returns the original error, allowing errors.Is and errors.As to work.
func (e *CustomErr) Unwrap() error {
	return e.Cause
}

// NewCustomErr creates a new CustomErr and captures the stack trace.
// If cause is nil, the returned error still carries a stack trace.
func NewCustomErr(code ErrorCode, message string, status int, cause error) error {
	return errors.WithStack(&CustomErr{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Cause:      cause,
	})
}

// NewCustomErrf creates a new CustomErr with a formatted message and captures the stack trace.
func NewCustomErrf(code ErrorCode, status int, cause error, format string, args ...any) error {
	return NewCustomErr(code, fmt.Sprintf(format, args...), status, cause)
}

// UnexpectedErrorMessage message for unexpecter app errors, usualy leads to returning InternalServerError
const UnexpectedErrorMessage = "Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"

// ErrorClassification defines error class for specific handling
type ErrorClassification int

const (
	// ErrorNonRetryable non retryable errors
	ErrorNonRetryable ErrorClassification = iota
	// ErrorRetryable errors
	ErrorRetryable
)

// ClassifyPostgreSQLError detects class of error which may happen during Postgres DB interaction
func ClassifyPostgreSQLError(err error) ErrorClassification {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.SerializationFailure,
			pgerrcode.DeadlockDetected,
			pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.InsufficientResources,
			pgerrcode.TooManyConnections,
			pgerrcode.LockNotAvailable:
			return ErrorRetryable
		}
	}
	var pgConnectErr *pgconn.ConnectError
	if errors.As(err, &pgConnectErr) {
		return ErrorRetryable
	}
	return ErrorNonRetryable
}
