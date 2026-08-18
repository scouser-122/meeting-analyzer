package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// Storage is an in-memory implementation of storage.FileStorage for tests.
type Storage struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewStorage creates a new in-memory storage instance.
func NewStorage() *Storage {
	return &Storage{data: make(map[string][]byte)}
}

// Save stores the content under the given key.
func (s *Storage) Save(ctx context.Context, key string, content io.Reader, size int64, contentType string) error {
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, content); err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = buf.Bytes()
	return nil
}

// Open returns a reader for the object identified by key.
func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// Delete removes the object identified by key.
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// Get returns the stored content for the given key.
func (s *Storage) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.data[key]
	return data, ok
}

// GetAll returns a copy of all stored content keyed by object key.
func (s *Storage) GetAll() map[string][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]byte, len(s.data))
	for k, v := range s.data {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
