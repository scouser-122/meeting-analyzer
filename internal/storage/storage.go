package storage

import (
	"context"
	"io"
)

// FileStorage defines the contract for storing and retrieving meeting audio files.
type FileStorage interface {
	// Save stores the content under the given key.
	Save(ctx context.Context, key string, content io.Reader, size int64, contentType string) error
	// Open returns a reader for the object identified by key.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the object identified by key.
	Delete(ctx context.Context, key string) error
}
