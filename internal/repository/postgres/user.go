package postgres

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
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

// NewPostgresUserRepositoryFromPool creates Postgres users storage from pool interface (for testing with pgxmock)
func NewPostgresUserRepositoryFromPool(pool QueryExecutor) *PostgresUserRepository {
	mapper := func(row pgx.Row) (*model.User, error) {
		var user model.User
		err := row.Scan(
			&user.ID,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &user, err
	}
	return &PostgresUserRepository{
		Database: &PostgresDatabase{},
		repo:     NewGenericRepositoryFromExecutor(pool, "users", "id", mapper),
	}
}

// Create creates a new user with the specified identifier.
func (s *PostgresUserRepository) Create(ctx context.Context, id string) (*model.User, error) {
	user, err := s.repo.Create(ctx, "id,created_at", id, time.Now())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				err = &models.CustomErr{Message: "user id busy", HTTPStatus: http.StatusConflict}
				return nil, errors.WithStack(err)
			}
		}
		return nil, errors.WithStack(err)
	}
	return user, nil
}

// GetByID returns a user by identifier.
func (s *PostgresUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Error("user not found", "id", id)
			return nil, fmt.Errorf("user not found")
		}
		return nil, errors.WithStack(err)
	}
	return user, nil
}
