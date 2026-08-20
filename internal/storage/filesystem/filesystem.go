package filesystem

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// Storage is a filesystem-backed implementation of storage.FileStorage.
type Storage struct {
	baseDir string
}

// NewStorage creates a new filesystem storage instance.
func NewStorage(baseDir string) *Storage {
	return &Storage{baseDir: baseDir}
}

// Save stores the content under the given key on the local filesystem.
func (s *Storage) Save(ctx context.Context, key string, content io.Reader, size int64, contentType string) error {
	destPath := filepath.Join(s.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return models.NewCustomErr(models.ErrCodeFileUploadFailed, "Не удалось сохранить файл. Попробуйте позже", http.StatusInternalServerError, err)
	}

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return models.NewCustomErr(models.ErrCodeFileUploadFailed, "Не удалось сохранить файл. Попробуйте позже", http.StatusInternalServerError, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, content); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return models.NewCustomErr(models.ErrCodeFileTooLarge, "Файл слишком большой. Уменьшите размер и попробуйте снова", http.StatusRequestEntityTooLarge, err)
		}
		return models.NewCustomErr(models.ErrCodeFileUploadFailed, "Не удалось сохранить файл. Попробуйте позже", http.StatusInternalServerError, err)
	}
	return nil
}

// Open returns a reader for the file identified by key.
func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.baseDir, key)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, models.NewCustomErr(models.ErrCodeFileOpenFailed, "Не удалось открыть файл. Попробуйте позже", http.StatusInternalServerError, err)
	}
	return file, nil
}

// Delete removes the file identified by key.
func (s *Storage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(s.baseDir, key)
	return errors.WithStack(os.RemoveAll(filePath))
}
