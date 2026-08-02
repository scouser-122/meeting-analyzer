package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// PostgresTaskRepository implements TaskRepositoru interface to store tasks data in Postgres DB
type PostgresTaskRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.Task]
}

// NewPostgresTaskRepository creates Postgres tasks storage
func NewPostgresTaskRepository(db *PostgresDatabase) *PostgresTaskRepository {
	mapper := func(row pgx.Row) (*model.Task, error) {
		var task model.Task
		err := row.Scan(
			&task.ID,
			&task.MeetingID,
			&task.Status,
			&task.ErrorMessage,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &task, err
	}
	return &PostgresTaskRepository{
		Database: db,
		repo:     NewGenericRepository(db, "tasks", "id", mapper),
	}
}

func (r *PostgresTaskRepository) Create(ctx context.Context, meetingID string) (*model.Task, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	id := uuid.New().String()
	task, err := repo.Create(
		ctx,
		"id,meeting_id,status,created_at,updated_at",
		id, meetingID, model.TaskStatusCreated, time.Now(), time.Now(),
	)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return task, nil
}

func (r *PostgresTaskRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	task, err := r.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &models.CustomErr{Message: "task not found", HTTPStatus: http.StatusBadRequest}
		}
		logger.Error(err.Error())
		return nil, err
	}
	return task, nil
}

func (r *PostgresTaskRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Task, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	task, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("task not found")
		}
		logger.Error(err.Error())
		return nil, err
	}
	return task, nil
}

func (r *PostgresTaskRepository) UpdateStatus(ctx context.Context, id string, status model.TaskStatus, errorMessage *string) error {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Update(
		ctx,
		"UPDATE tasks SET status = $1, updated_at = $2, error_message = $3 WHERE id = $4",
		status,
		time.Now(),
		errorMessage,
		id,
	)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return err
}
