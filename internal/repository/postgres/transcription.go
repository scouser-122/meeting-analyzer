package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// PostgresTranscriptionRepository implements TaskRepositoru interface to store tasks data in Postgres DB
type PostgresTranscriptionRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.Transcription]
}

// NewPostgresTaskRepository creates Postgres tasks storage
func NewPostgresTranscriptionRepository(db *PostgresDatabase) *PostgresTranscriptionRepository {
	mapper := func(row pgx.Row) (*model.Transcription, error) {
		var transcription model.Transcription
		err := row.Scan(
			&transcription.ID,
			&transcription.MeetingID,
			&transcription.Text,
			&transcription.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &transcription, err
	}
	return &PostgresTranscriptionRepository{
		Database: db,
		repo:     NewGenericRepository(db, "transcriptions", "id", mapper),
	}
}

func (r *PostgresTranscriptionRepository) Create(ctx context.Context, transcription *model.Transcription) error {
	logger := logger.GetSlogLoggerFromContext(ctx)
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	transcription, err := repo.Create(
		ctx,
		"id,meeting_id,text,created_at",
		transcription.ID, transcription.MeetingID, transcription.Text, time.Now(),
	)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return nil
}

func (r *PostgresTranscriptionRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	transcription, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("transcription not found")
		}
		logger.Error(err.Error())
		return nil, err
	}
	return transcription, nil
}
