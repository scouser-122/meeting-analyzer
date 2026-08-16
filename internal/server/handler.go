package server

import (
	"errors"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
	"github.com/scouser-122/meeting-analyzer/internal/worker"
)

// Handler represents an HTTP handler configuration with method, path pattern, and handler function.
type Handler struct {
	URLPathPattern string
	HandlerFn      http.HandlerFunc
}

// InitializeHandlers creates and returns a slice of all HTTP handlers for the metrics service.
// It configures both update and read handlers with their respective routes.
func InitializeHandlers(
	serverConfig *config.ServerConfig,
	usersService *service.UsersService,
	meetingsService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	meetingProcessor *worker.MeetingProcessor,
	llmClient client.LLMClient,
) []Handler {
	handlers := []Handler{}

	usersHandler := NewUsersHandler(usersService)
	handlers = append(handlers, Handler{"/api/users/start", usersHandler.HandleStart})

	meetingsHandler := NewMeetingsHandler(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		meetingProcessor,
		serverConfig,
	)
	handlers = append(handlers, Handler{"/api/meetings/load", meetingsHandler.HandleLoad})
	handlers = append(handlers, Handler{"/api/meetings/list", meetingsHandler.HandleList})
	handlers = append(handlers, Handler{"/api/meetings/status", meetingsHandler.HandleStatus})
	handlers = append(handlers, Handler{"/api/meetings/transcription", meetingsHandler.HandleTranscription})
	handlers = append(handlers, Handler{"/api/meetings/find", meetingsHandler.HandleFind})
	handlers = append(handlers, Handler{"/api/meetings/delete", meetingsHandler.HandleDelete})

	chatHandler := NewChatHandler(
		llmClient,
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
	)
	handlers = append(handlers, Handler{"/api/chat", chatHandler.HandleChat})

	return handlers
}

func handleServiceError(err error, res http.ResponseWriter) {
	var customErr *models.CustomErr
	if errors.As(err, &customErr) {
		models.WriteResponseError(customErr, res)
		return
	} else {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}
}
