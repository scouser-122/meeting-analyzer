package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Session struct {
	UserID string `json:"user_id"`
}

func sessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".meeting-analyzer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create session dir: %w", err)
	}
	return dir, nil
}

func sessionPath() (string, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", errors.WithStack(err)
	}
	return filepath.Join(dir, "session.json"), nil
}

func LoadSession() (*Session, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}

	if _, err := uuid.Parse(sess.UserID); err != nil {
		return nil, fmt.Errorf("invalid user_id in session: %w", err)
	}

	return &sess, nil
}

func SaveSession(userID string) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}

	sess := Session{UserID: userID}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

func NewUserID() string {
	return uuid.New().String()
}
