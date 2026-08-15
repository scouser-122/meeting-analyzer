package service

import (
	"context"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
	"github.com/scouser-122/meeting-analyzer/internal/models"
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
func (s *UsersService) Register(ctx context.Context, id string) error {
	if id == "" {
		return &models.CustomErr{Message: "user id absent", HTTPStatus: http.StatusBadRequest}
	}
	_, err := s.usersRepo.Create(ctx, id)
	if err != nil {
		return err
	}
	return nil
}

// GetByID returns a user by identifier.
func (s *UsersService) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.usersRepo.GetByID(ctx, id)
}
