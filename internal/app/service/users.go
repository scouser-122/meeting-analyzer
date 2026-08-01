package service

import (
	"context"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
)

// UsersService service to work with users
type UsersService struct {
	usersRepo repository.UserRepository
}

// NewUsersService creates new UsersService instance
func NewUsersService(
	usersRepo repository.UserRepository,
) *UsersService {
	service := UsersService{}
	service.usersRepo = usersRepo
	return &service
}

// Register runs registration process for specified user
func (service *UsersService) Register(ctx context.Context, id string) error {
	if id == "" {
		return &models.CustomErr{Message: "user id absent", HTTPStatus: http.StatusBadRequest}
	}
	_, err := service.usersRepo.Create(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
