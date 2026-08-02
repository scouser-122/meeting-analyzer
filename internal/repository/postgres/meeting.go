package postgres

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// PostgresMeetingRepository implements MeetingRepository interface to store meetings data in Postgres DB
type PostgresMeetingRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.Meeting]
}

// NewPostgresMeetingRepository creates Postgres meetings storage
func NewPostgresMeetingRepository(db *PostgresDatabase) *PostgresMeetingRepository {
	mapper := func(row pgx.Row) (*model.Meeting, error) {
		var meeting model.Meeting
		err := row.Scan(
			&meeting.ID,
			&meeting.UserID,
			&meeting.Name,
			&meeting.FilePath,
			&meeting.OriginalFilename,
			&meeting.CreatedAt,
			&meeting.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &meeting, err
	}
	return &PostgresMeetingRepository{
		Database: db,
		repo:     NewGenericRepository(db, "meetings", "id", mapper),
	}
}

func (r *PostgresMeetingRepository) Create(ctx context.Context, userID string) (*model.Meeting, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	id := uuid.New().String()
	meeting, err := repo.Create(ctx, "id,user_id,created_at,updated_at", id, userID, time.Now(), time.Now())
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return meeting, nil
}

func (r *PostgresMeetingRepository) GetByID(ctx context.Context, id string) (*model.Meeting, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	meeting, err := r.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &models.CustomErr{Message: "meeting not found", HTTPStatus: http.StatusBadRequest}
		}
		logger.Error(err.Error())
		return nil, err
	}
	return meeting, nil
}

func (r *PostgresMeetingRepository) Update(ctx context.Context, meeting *model.Meeting) error {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Update(
		ctx,
		"UPDATE meetings SET name = $1, file_path = $2, original_file_name = $3, updated_at = $4 WHERE id = $5",
		meeting.Name,
		meeting.FilePath,
		meeting.OriginalFilename,
		time.Now(),
		meeting.ID,
	)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return err
}

const meetingsPageSize = 10

func (r *PostgresMeetingRepository) GetByUserID(ctx context.Context, userID string) ([]*model.Meeting, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	result := []*model.Meeting{}
	for orders, err := range r.repo.GetAllConditional(
		ctx,
		"WHERE user_id = $1",
		[]any{userID},
		"created_at DESC",
		meetingsPageSize,
	) {
		if err != nil {
			logger.Error(err.Error())
			return []*model.Meeting{}, err
		}
		result = append(result, orders...)
	}
	return result, nil
}
