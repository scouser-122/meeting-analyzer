package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/app/service"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
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

// HandleRegister processes user registration request
func (h *UsersHandler) HandleRegister(res http.ResponseWriter, req *http.Request) {
	logger := logger.GetSlogLoggerFromContext(req.Context())

	bodyBuf, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("cannot read request body", "err", err)
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var user model.User
	if err := json.Unmarshal(bodyBuf, &user); err != nil {
		logger.Error("cannot decode request json body", "err", err)
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	err = h.usersService.Register(req.Context(), user.ID)
	if err != nil {
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

	// authToken, err := h.jwtService.GenerateJWT(registeredUser.Login)
	// if err != nil {
	// 	logger.Error("error generating JWT token", "err", err)
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
	// 	return
	// }

	// res.Header().Add("Authorization", fmt.Sprintf("Bearer %s", authToken))

	successMessage := "user successfully registered"
	logger.Info(successMessage, slog.String("id", user.ID))
	res.Header().Set("content-type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(models.NewSuccessResponseBuffer(successMessage))
}
