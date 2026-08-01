package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// PostgresUserRepository implements UserRepository interface to store users data in Postgres DB
type PostgresUserRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.User]
}

// NewPostgresUserRepository creates Postgres users storage
func NewPostgresUserRepository(db *PostgresDatabase) *PostgresUserRepository {
	mapper := func(row pgx.Row) (*model.User, error) {
		var user model.User
		err := row.Scan(
			&user.ID,
			&user.ExternalID,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &user, err
	}
	return &PostgresUserRepository{
		Database: db,
		repo:     NewGenericRepository(db, "users", "id", mapper),
	}
}

// Create creates new user,
// returns error if user with specified login already exists or process failed
func (s *PostgresUserRepository) Create(ctx context.Context, id string) (*model.User, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	user, err := s.repo.Create(ctx, "id,created_at", id, time.Now())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				err = &models.CustomErr{Message: "user id busy", HTTPStatus: http.StatusConflict}
				logger.Error(err.Error())
				return nil, err
			}
		}
		logger.Error(err.Error())
		return nil, err
	}
	return user, nil
}

// Get obtains user from storage by ID
func (s *PostgresUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		logger.Error(err.Error())
		return nil, err
	}
	return user, nil
}

// Get obtains user from storage by external ID
func (s *PostgresUserRepository) GetByExternalID(ctx context.Context, externalID string) (*model.User, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	user, err := s.repo.GetByParameter(ctx, "external_id", externalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		logger.Error(err.Error())
		return nil, err
	}
	return user, nil
}
