package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
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
			&summary.SearchV,
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
		return errors.WithStack(err)
	}
	return nil
}

func (r *PostgresSummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Summary, error) {
	summary, err := r.repo.GetByParameter(ctx, "meeting_id", meetingID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("summary not found")
		}
		return nil, errors.WithStack(err)
	}
	return summary, nil
}

func (r *PostgresSummaryRepository) FindByTextContains(ctx context.Context, userID string, textPart string) ([]*model.Summary, error) {
	result := []*model.Summary{}
	const sql = `
		SELECT id, meeting_id, user_id, text, created_at,
		       ts_rank(search_vector, websearch_to_tsquery('russian', $2)) AS rank
		FROM summary
		WHERE user_id = $1
		  AND search_vector @@ websearch_to_tsquery('russian', $2)
		ORDER BY rank DESC
		LIMIT 5
	`
	r.repo.CustomQuery(
		ctx,
		func(rows pgx.Rows) error {
			for rows.Next() {
				var summary model.Summary
				var rank float64
				err := rows.Scan(
					&summary.ID,
					&summary.MeetingID,
					&summary.UserID,
					&summary.Text,
					&summary.CreatedAt,
					&rank,
				)
				if err != nil {
					return errors.WithStack(err)
				}
				result = append(result, &summary)
			}
			return nil
		},
		sql,
		userID,
		textPart,
	)
	return result, nil
}
