package postgres

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
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

// Create creates a new processing task for the specified meeting.
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
		logger.Error("can't create new task", "err", err)
		return nil, err
	}
	return task, nil
}

// GetByID returns a task by its identifier.
func (r *PostgresTaskRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	task, err := r.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Error("task not found")
			return nil, models.NewCustomErr(models.ErrCodeTaskNotFound, "Задача обработки не найдена", http.StatusBadRequest, err)
		}
		return nil, errors.WithStack(err)
	}
	return task, nil
}

// GetByMeetingID returns the task associated with the specified meeting.
func (r *PostgresTaskRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Task, error) {
	task, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.NewCustomErr(models.ErrCodeTaskNotFound, "Задача обработки не найдена", http.StatusNotFound, err)
		}
		return nil, errors.WithStack(err)
	}
	return task, nil
}

// UpdateStatus updates the status and optional error message of a task.
func (r *PostgresTaskRepository) UpdateStatus(ctx context.Context, id string, status model.TaskStatus, errorMessage *string) error {
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
		return errors.WithStack(err)
	}
	return nil
}

// DeleteByMeetingID removes the task associated with the specified meeting.
func (r *PostgresTaskRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Update(
		ctx,
		"DELETE FROM tasks WHERE meeting_id = $1",
		meetingID,
	)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// NewPostgresTaskRepositoryFromPool creates Postgres tasks storage from pool interface (for testing with pgxmock)
func NewPostgresTaskRepositoryFromPool(pool QueryExecutor) *PostgresTaskRepository {
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
		Database: &PostgresDatabase{},
		repo:     NewGenericRepositoryFromExecutor(pool, "tasks", "id", mapper),
	}
}
