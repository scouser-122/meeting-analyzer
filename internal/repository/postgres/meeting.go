package postgres

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
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
			&meeting.MeetingName,
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
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	id := uuid.New().String()
	meeting, err := repo.Create(ctx, "id,user_id,created_at,updated_at", id, userID, time.Now(), time.Now())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return meeting, nil
}

func (r *PostgresMeetingRepository) GetByID(ctx context.Context, id string) (*model.Meeting, error) {
	meeting, err := r.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &models.CustomErr{Message: "meeting not found", HTTPStatus: http.StatusNotFound}
		}
		return nil, errors.WithStack(err)
	}
	return meeting, nil
}

func (r *PostgresMeetingRepository) Update(ctx context.Context, meeting *model.Meeting) error {
	repo := r.repo
	tx := models.GetTransactionFromContext(ctx)
	if tx != nil {
		repo = r.repo.WithTx(tx.(pgx.Tx))
	}
	_, err := repo.Update(
		ctx,
		"UPDATE meetings SET meeting_name = $1, file_path = $2, original_file_name = $3, updated_at = $4 WHERE id = $5",
		meeting.MeetingName,
		meeting.FilePath,
		meeting.OriginalFilename,
		time.Now(),
		meeting.ID,
	)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

const meetingsPageSize = 10

func (r *PostgresMeetingRepository) GetByUserID(ctx context.Context, userID string) ([]*model.Meeting, error) {
	result := []*model.Meeting{}
	for meetings, err := range r.repo.GetAllConditional(
		ctx,
		"WHERE user_id = $1",
		[]any{userID},
		"created_at DESC",
		meetingsPageSize,
	) {
		if err != nil {
			return []*model.Meeting{}, errors.WithStack(err)
		}
		result = append(result, meetings...)
	}
	return result, nil
}

func (r *PostgresMeetingRepository) FindByNameContains(ctx context.Context, userID string, namePart string) ([]*model.Meeting, error) {
	result := []*model.Meeting{}
	for meetings, err := range r.repo.GetAllConditional(
		ctx,
		"WHERE user_id = $1 AND meeting_name LIKE '%' || $2::text || '%'",
		[]any{userID, namePart},
		"created_at DESC",
		meetingsPageSize,
	) {
		if err != nil {
			return []*model.Meeting{}, errors.WithStack(err)
		}
		result = append(result, meetings...)
	}
	return result, nil
}
