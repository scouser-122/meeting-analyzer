package server

import (
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/app/service"
	"github.com/scouser-122/meeting-analyzer/internal/config"
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
) []Handler {
	handlers := []Handler{}

	usersHandler := NewUsersHandler(usersService)
	handlers = append(handlers, Handler{"/api/users/register", usersHandler.HandleRegister})

	meetingsHandler := NewMeetingsHandler(meetingsService, serverConfig)
	handlers = append(handlers, Handler{"/api/meetings/load", meetingsHandler.HandleLoad})

	return handlers
}
