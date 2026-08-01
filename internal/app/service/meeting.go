package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// MeetingsService service to work with meetings
type MeetingsService struct {
	meetingsRepo    repository.MeetingRepository
	usersRepo       repository.UserRepository
	repositoryUtils repository.RepositoryUtils
	uploadDir       string
}

// MeetingsService creates new MeetingsService instance
func NewMeetingsService(
	meetingsRepo repository.MeetingRepository,
	usersRepo repository.UserRepository,
	repositoryUtils repository.RepositoryUtils,
	serverConfig *config.ServerConfig,
) *MeetingsService {
	service := MeetingsService{}
	service.meetingsRepo = meetingsRepo
	service.usersRepo = usersRepo
	service.repositoryUtils = repositoryUtils
	service.uploadDir = *serverConfig.UploadFileDir
	return &service
}

var allowedExts = map[string]bool{
	".mp3": true,
	".wav": true,
	".m4a": true,
	".ogg": true,
}

// Load runs registration process for specified user
func (s *MeetingsService) Load(
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

	_, err := s.usersRepo.GetByID(ctx, meeting.UserID)
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

	if meeting.Name != nil {
		newMeeting.Name = new(string)
		*newMeeting.Name = *meeting.Name
	}

	err = s.saveMeetingFileToFS(ctx, newMeeting, file, header)
	if err != nil {
		return nil, err
	}

	err = s.meetingsRepo.Update(ctx, newMeeting)
	if err != nil {
		return nil, err
	}

	err = s.repositoryUtils.CommitTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}
	return newMeeting, nil
}

func (s *MeetingsService) saveMeetingFileToFS(
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

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		logger.Error("mkdir failed", "err", err)
		return &models.CustomErr{
			Message:    "internal error",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	destPath := filepath.Join(s.uploadDir, fileID+ext)
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		logger.Error("create dest file failed", "err", err)
		return &models.CustomErr{
			Message:    "internal error",
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		os.Remove(destPath)
		if err.Error() == "http: request body too large" {
			return &models.CustomErr{
				Message:    "file too large",
				HTTPStatus: http.StatusRequestEntityTooLarge,
			}
		}
		logger.Error("copy failed", "err", err)
		return &models.CustomErr{
			Message:    "upload failed",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	meeting.FilePath = new(string)
	*meeting.FilePath = destPath
	meeting.OriginalFilename = new(string)
	*meeting.OriginalFilename = header.Filename

	return nil
}

func newFileID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
