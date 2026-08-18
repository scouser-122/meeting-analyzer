package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/storage"
)

// MeetingsService service to work with meetings
type MeetingsService struct {
	meetingsRepo         repository.MeetingRepository
	repositoryUtils      repository.RepositoryUtils
	usersService         *UsersService
	tasksService         *TasksService
	transcriptionService *TranscriptionService
	summaryService       *SummaryService
	fileStorage          storage.FileStorage
	serverConfig         *config.ServerConfig
}

// NewMeetingsService creates new MeetingsService instance.
func NewMeetingsService(
	meetingsRepo repository.MeetingRepository,
	repositoryUtils repository.RepositoryUtils,
	usersService *UsersService,
	tasksService *TasksService,
	transcriptionService *TranscriptionService,
	summaryService *SummaryService,
	serverConfig *config.ServerConfig,
	fileStorage storage.FileStorage,
) *MeetingsService {
	service := MeetingsService{}
	service.meetingsRepo = meetingsRepo
	service.repositoryUtils = repositoryUtils
	service.usersService = usersService
	service.tasksService = tasksService
	service.transcriptionService = transcriptionService
	service.summaryService = summaryService
	service.serverConfig = serverConfig
	service.fileStorage = fileStorage
	return &service
}

var allowedExts = map[string]bool{
	".mp3": true,
	".wav": true,
	".m4a": true,
	".ogg": true,
}

// CreateFromAudioFile saves the uploaded audio file and creates a meeting record with a processing task.
func (s *MeetingsService) CreateFromAudioFile(
	ctx context.Context,
	meeting *model.Meeting,
	file multipart.File,
	header *multipart.FileHeader,
) (*model.Meeting, error) {
	if meeting.UserID == "" {
		return nil, &models.CustomErr{
			Message:    "user id not specified",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	_, err := s.usersService.GetByID(ctx, meeting.UserID)
	if err != nil {
		return nil, err
	}

	tx, err := s.repositoryUtils.CreateTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ctx = context.WithValue(ctx, models.DbTransactionKey, tx)

	newMeeting, err := s.meetingsRepo.Create(ctx, meeting.UserID)
	if err != nil {
		return nil, err
	}

	if meeting.MeetingName != nil {
		newMeeting.MeetingName = new(string)
		*newMeeting.MeetingName = *meeting.MeetingName
	}

	err = s.saveMeetingFile(ctx, newMeeting, file, header)
	if err != nil {
		return nil, err
	}

	err = s.meetingsRepo.Update(ctx, newMeeting)
	if err != nil {
		return nil, err
	}

	_, err = s.tasksService.CreateNewTask(ctx, newMeeting.ID)
	if err != nil {
		return nil, err
	}

	err = s.repositoryUtils.CommitTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}
	return newMeeting, nil
}

func (s *MeetingsService) saveMeetingFile(
	ctx context.Context,
	meeting *model.Meeting,
	file multipart.File,
	header *multipart.FileHeader,
) error {
	logger := logger.GetSlogLoggerFromContext(ctx)

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		logger.Error("unsupported file type", "ext", ext)
		return &models.CustomErr{
			Message:    fmt.Sprintf("unsupported file type: %s", ext),
			HTTPStatus: http.StatusUnsupportedMediaType,
		}
	}

	fileID, err := newFileID()
	if err != nil {
		logger.Error("fileID generation failed", "err", err)
		return &models.CustomErr{
			Message:    "internal error",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	key := s.buildFileKey(meeting, fileID, ext)

	err = s.fileStorage.Save(ctx, key, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		logger.Error("save meeting file failed", "err", err)
		var customErr *models.CustomErr
		if errors.As(err, &customErr) {
			return err
		}
		return &models.CustomErr{
			Message:    "upload failed",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	meeting.FilePath = &key
	meeting.OriginalFilename = &header.Filename

	return nil
}

func (s *MeetingsService) buildFileKey(meeting *model.Meeting, fileID, ext string) string {
	return fmt.Sprintf("%s/%s%s", meeting.UserID, fileID, ext)
}

func newFileID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// DeleteMeetingAudioFile removes the meeting audio file from storage and clears the file path in storage.
func (s *MeetingsService) DeleteMeetingAudioFile(
	ctx context.Context,
	meeting *model.Meeting,
) error {
	if meeting.FilePath == nil || *meeting.FilePath == "" {
		return nil
	}
	err := s.fileStorage.Delete(ctx, *meeting.FilePath)
	if err != nil {
		return errors.WithStack(err)
	}
	meeting.FilePath = nil
	err = s.meetingsRepo.Update(ctx, meeting)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// GetByID returns a meeting by identifier.
func (s *MeetingsService) GetByID(ctx context.Context, meetingID string) (*model.Meeting, error) {
	return s.meetingsRepo.GetByID(ctx, meetingID)
}

// GetAllByUserID returns all meetings owned by the user.
func (s *MeetingsService) GetAllByUserID(ctx context.Context, userID string) ([]*model.Meeting, error) {
	return s.meetingsRepo.GetByUserID(ctx, userID)
}

// FileStorage returns the underlying file storage used by the service.
func (s *MeetingsService) FileStorage() storage.FileStorage {
	return s.fileStorage
}

// FindByNameContains searches user meetings by a substring of the meeting name.
func (s *MeetingsService) FindByNameContains(ctx context.Context, userID string, namePart string) ([]*model.Meeting, error) {
	return s.meetingsRepo.FindByNameContains(ctx, userID, namePart)
}

// Delete removes a meeting and its associated audio file if the user is the owner.
func (s *MeetingsService) Delete(ctx context.Context, userID, meetingID string) error {
	meeting, err := s.meetingsRepo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}

	if meeting.UserID != userID {
		return &models.CustomErr{
			Message:    "meeting data belongs to another user",
			HTTPStatus: http.StatusForbidden,
		}
	}

	if meeting.FilePath != nil && *meeting.FilePath != "" {
		if err := s.fileStorage.Delete(ctx, *meeting.FilePath); err != nil {
			return errors.WithStack(err)
		}
	}

	if err := s.tasksService.DeleteByMeetingID(ctx, meetingID); err != nil {
		return errors.WithStack(err)
	}

	if err := s.transcriptionService.DeleteByMeetingID(ctx, meetingID); err != nil {
		return errors.WithStack(err)
	}

	if err := s.summaryService.DeleteByMeetingID(ctx, meetingID); err != nil {
		return errors.WithStack(err)
	}

	return s.meetingsRepo.Delete(ctx, meetingID)
}
