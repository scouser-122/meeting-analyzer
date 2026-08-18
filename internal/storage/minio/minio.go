package minio

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// Storage is a MinIO-backed implementation of storage.FileStorage.
type Storage struct {
	client *minio.Client
	bucket string
	prefix string
}

// NewStorage creates a new MinIO storage instance from the provided config.
func NewStorage(cfg *config.MinioConfig) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Storage{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// fullKey returns the object key with the configured prefix.
func (s *Storage) fullKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

// Save uploads the content to MinIO under the given key.
func (s *Storage) Save(ctx context.Context, key string, content io.Reader, size int64, contentType string) error {
	fullKey := s.fullKey(key)
	_, err := s.client.PutObject(ctx, s.bucket, fullKey, content, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return &models.CustomErr{
			Message:    fmt.Sprintf("failed to save file to minio: %v", err),
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	return nil
}

// Open returns a reader for the object identified by key.
func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)
	obj, err := s.client.GetObject(ctx, s.bucket, fullKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, &models.CustomErr{
			Message:    fmt.Sprintf("failed to open file from minio: %v", err),
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	return obj, nil
}

// Delete removes the object identified by key.
func (s *Storage) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)
	err := s.client.RemoveObject(ctx, s.bucket, fullKey, minio.RemoveObjectOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
