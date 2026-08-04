package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// PostgresTranscriptionRepository implements TaskRepositoru interface to store tasks data in Postgres DB
type PostgresSummaryRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.Summary]
}

// NewPostgresTaskRepository creates Postgres tasks storage
func NewPostgresSummaryRepository(db *PostgresDatabase) *PostgresSummaryRepository {
	mapper := func(row pgx.Row) (*model.Summary, error) {
		var summary model.Summary
		err := row.Scan(
			&summary.ID,
			&summary.MeetingID,
			&summary.UserID,
			&summary.Text,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &summary, err
	}
	return &PostgresSummaryRepository{
		Database: db,
		repo:     NewGenericRepository(db, "summary", "id", mapper),
	}
}

func (r *PostgresSummaryRepository) Create(ctx context.Context, summary *model.Summary) error {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Create(
		ctx,
		"id,meeting_id,user_id,text,created_at",
		summary.ID, summary.MeetingID, summary.UserID, summary.Text, time.Now(),
	)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return nil
}

func (r *PostgresSummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Summary, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	summary, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("summary not found")
		}
		logger.Error(err.Error())
		return nil, err
	}
	return summary, nil
}

func (r *PostgresSummaryRepository) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Summary, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	result := []*model.Summary{}
	for summaries, err := range r.repo.GetAllConditional(
		ctx,
		"WHERE user_id = $1 AND text LIKE '%$2%'",
		[]any{userID, textPart},
		"created_at DESC",
		meetingsPageSize,
	) {
		if err != nil {
			logger.Error(err.Error())
			return []*model.Summary{}, err
		}
		result = append(result, summaries...)
	}
	return result, nil
}
