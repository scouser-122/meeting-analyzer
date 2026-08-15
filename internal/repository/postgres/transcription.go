package postgres

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// PostgresTranscriptionRepository implements TranscriptionRepository interface to store transcription data in Postgres DB.
type PostgresTranscriptionRepository struct {
	Database *PostgresDatabase
	repo     *GenericRepository[model.Transcription]
}

// NewPostgresTranscriptionRepository creates Postgres transcription storage.
func NewPostgresTranscriptionRepository(db *PostgresDatabase) *PostgresTranscriptionRepository {
	mapper := func(row pgx.Row) (*model.Transcription, error) {
		var transcription model.Transcription
		err := row.Scan(
			&transcription.ID,
			&transcription.MeetingID,
			&transcription.UserID,
			&transcription.Text,
			&transcription.CreatedAt,
			&transcription.SearchV,
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

// Create persists a new transcription record in the database.
func (r *PostgresTranscriptionRepository) Create(ctx context.Context, transcription *model.Transcription) error {
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Create(
		ctx,
		"id,meeting_id,user_id,text,created_at",
		transcription.ID, transcription.MeetingID, transcription.UserID, transcription.Text, time.Now(),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// GetByMeetingID returns the transcription associated with the specified meeting.
func (r *PostgresTranscriptionRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error) {
	transcription, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &models.CustomErr{Message: "transcription not found", HTTPStatus: http.StatusNotFound}
		}
		return nil, errors.WithStack(err)
	}
	return transcription, nil
}

// FindByTextContains searches transcriptions by text using full-text search.
func (r *PostgresTranscriptionRepository) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Transcription, error) {
	result := []*model.Transcription{}
	const sql = `
		SELECT id, meeting_id, user_id, text, created_at,
		       ts_rank(search_vector, websearch_to_tsquery('russian', $2)) AS rank
		FROM transcriptions
		WHERE user_id = $1
		  AND search_vector @@ websearch_to_tsquery('russian', $2)
		ORDER BY rank DESC
		LIMIT 5
	`
	r.repo.CustomQuery(
		ctx,
		func(rows pgx.Rows) error {
			for rows.Next() {
				var transcription model.Transcription
				var rank float64
				err := rows.Scan(
					&transcription.ID,
					&transcription.MeetingID,
					&transcription.UserID,
					&transcription.Text,
					&transcription.CreatedAt,
					&rank,
				)
				if err != nil {
					return errors.WithStack(err)
				}
				result = append(result, &transcription)
			}
			return nil
		},
		sql,
		userID,
		textPart,
	)
	return result, nil
}

// FindByKeyWords searches transcriptions by keywords and topic using full-text search.
func (r *PostgresTranscriptionRepository) FindByKeyWords(ctx context.Context, userID string, keywords []string, topic string) ([]*model.Transcription, error) {
	result := []*model.Transcription{}
	terms := append(append([]string{}, keywords...), topic)
	tsQuery := strings.Join(terms, " | ")
	safeQuery := buildTsQuery(terms)
	const sql = `
		SELECT id, meeting_id, user_id, text, created_at,
		       ts_rank(search_vector, plainto_tsquery('russian', $2)) AS rank
		FROM transcriptions
		WHERE user_id = $1
		  AND search_vector @@ to_tsquery('russian', $3)
		ORDER BY rank DESC
		LIMIT 5
	`
	r.repo.CustomQuery(
		ctx,
		func(rows pgx.Rows) error {
			for rows.Next() {
				var transcription model.Transcription
				var rank float64
				err := rows.Scan(
					&transcription.ID,
					&transcription.MeetingID,
					&transcription.UserID,
					&transcription.Text,
					&transcription.CreatedAt,
					&rank,
				)
				if err != nil {
					return errors.WithStack(err)
				}
				result = append(result, &transcription)
			}
			return nil
		},
		sql,
		userID,
		tsQuery,
		safeQuery,
	)
	return result, nil

}

func buildTsQuery(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(t, "'", "''")+"'")
	}
	return strings.Join(quoted, " | ")
}
