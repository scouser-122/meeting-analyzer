package telegram

import (
	"fmt"

	"github.com/google/uuid"
)

// namespace is a fixed namespace UUID used to derive a deterministic user ID
// from a Telegram numeric ID. It is kept stable so that the same Telegram user
// always maps to the same UUID, regardless of bot restarts.
var namespace = uuid.MustParse("9f8e7d6c-5b4a-3c2d-1e0f-0a1b2c3d4e5f")

// UserIDFromTelegramID converts a Telegram numeric user ID into a deterministic UUID.
// This satisfies the backend requirement that USER_ID be in UUID format, while
// remaining stable across bot restarts.
func UserIDFromTelegramID(telegramID int64) string {
	name := fmt.Sprintf("telegram:%d", telegramID)
	return uuid.NewSHA1(namespace, []byte(name)).String()
}
