package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// UsersHandler specifies http request handler for requests to Users service
type UsersHandler struct {
	usersService *service.UsersService
}

// NewUsersHandler creates and returns pointer to new UsersHandler
func NewUsersHandler(
	usersService *service.UsersService,
) *UsersHandler {
	return &UsersHandler{
		usersService: usersService,
	}
}

// HandleStart processes user registration request
func (h *UsersHandler) HandleStart(res http.ResponseWriter, req *http.Request) {
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	bodyBuf, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("cannot read request body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var user model.User
	if err := json.Unmarshal(bodyBuf, &user); err != nil {
		logger.Error("cannot decode request json body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	err = h.usersService.Register(req.Context(), user.ID)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	successMessage := "user successfully registered"
	logger.Info(successMessage, slog.String("id", user.ID))
	res.WriteHeader(http.StatusOK)
	res.Write(models.NewSuccessResponseBuffer(successMessage))
}
