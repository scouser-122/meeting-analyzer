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
	repo      repository.MeetingRepository
	uploadDir string
}

// MeetingsService creates new MeetingsService instance
func NewMeetingsService(
	repo repository.MeetingRepository,
	serverConfig *config.ServerConfig,
) *MeetingsService {
	service := MeetingsService{}
	service.repo = repo
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
) error {
	logger := logger.GetSlogLoggerFromContext(ctx)

	newMeeting, err := s.repo.Create(ctx, meeting.UserID)
	if err != nil {
		return err
	}

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

	newMeeting.FilePath = new(string)
	*newMeeting.FilePath = destPath
	newMeeting.OriginalFilename = new(string)
	*newMeeting.OriginalFilename = header.Filename

	err = s.repo.Update(ctx, newMeeting)
	if err != nil {
		return err
	}
	return nil
}

func newFileID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
