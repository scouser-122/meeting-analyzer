package telegram

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	tele "gopkg.in/telebot.v4"
)

// TelegramBot wraps a Telegram bot that proxies user commands to the backend API.
type TelegramBot struct {
	config *TelegramBotConfig
	client *TelegramClient
	bot    *tele.Bot
}

// NewTelegramBot creates a new Telegram bot instance without initializing the underlying client.
func NewTelegramBot(config *TelegramBotConfig) *TelegramBot {
	return &TelegramBot{
		config: config,
	}
}

// Init creates the backend client and registers Telegram handlers.
func (b *TelegramBot) Init() error {
	b.client = NewClient(b.config.Backend.BaseURL)

	pref := tele.Settings{
		Token:  b.config.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return errors.WithStack(err)
	}

	bot.Handle("/start", b.handleStart)
	bot.Handle("/list", b.handleList)
	bot.Handle("/status", b.handleStatus)
	bot.Handle("/transcription", b.handleTranscription)
	bot.Handle("/retry", b.handleRetry)
	bot.Handle("/delete", b.handleDelete)
	bot.Handle("/find", b.handleFind)

	bot.Handle(tele.OnAudio, b.handleMedia)

	bot.Handle(tele.OnVoice, b.handleMedia)

	bot.Handle(tele.OnText, b.handleChat)

	b.bot = bot

	return nil
}

// Run starts the bot and blocks until it is stopped.
func (b *TelegramBot) Run() {
	slog.Info("telegram bot started")
	b.bot.Start()
}

func (b *TelegramBot) handleStart(c tele.Context) error {
	userID := UserIDFromTelegramID(c.Sender().ID)
	_, err := b.client.Start(userID)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка регистрации: %v", err))
	}
	return c.Send("Готов к обработке команд\nЗагрузите аудио файл с записью встречи для обработки, или выберите любую из команд, или задайте вопрос в чате.")
}

func (b *TelegramBot) handleMedia(c tele.Context) error {
	var media tele.File
	var caption string

	if c.Message().Audio != nil {
		media = c.Message().Audio.File
		caption = c.Message().Caption
	} else if c.Message().Voice != nil {
		media = c.Message().Voice.File
		caption = c.Message().Caption
	} else {
		return c.Send("Не удалось определить медиафайл.")
	}

	if caption == "" {
		caption = "Встреча без названия"
	}

	reader, err := c.Bot().File(&media)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка загрузки файла из Telegram: %v", err))
	}
	defer reader.Close()

	fileName := "audio"
	if c.Message().Audio != nil && c.Message().Audio.FileName != "" {
		fileName = c.Message().Audio.FileName
	}

	userID := UserIDFromTelegramID(c.Sender().ID)
	resp, err := b.client.UploadMeeting(userID, caption, fileName, reader)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка загрузки встречи: %v", err))
	}

	return c.Send(fmt.Sprintf("%s\nID встречи: %s", resp.Message, resp.Data.ID))
}

func (b *TelegramBot) handleList(c tele.Context) error {
	userID := UserIDFromTelegramID(c.Sender().ID)
	meetings, err := b.client.List(userID)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка получения списка: %v", err))
	}
	if len(meetings) == 0 {
		return c.Send("У вас пока нет сохранённых встреч.")
	}

	var sb strings.Builder
	sb.WriteString("Ваши встречи:\n\n")
	for _, m := range meetings {
		if m.Name != nil {
			sb.WriteString(fmt.Sprintf("ID: %s\nНазвание: %s\nСтатус: %s\nСоздана: %s\n", m.ID, *m.Name, m.Status, m.CreatedAt.Format(time.RFC3339)))
		} else {
			sb.WriteString(fmt.Sprintf("ID: %s\nСтатус: %s\nСоздана: %s\n", m.ID, m.Status, m.CreatedAt.Format(time.RFC3339)))
		}
		if m.Summary != nil {
			sb.WriteString(fmt.Sprintf("Выжимка: %s\n", *m.Summary))
		}
		sb.WriteString("\n")
	}
	return c.Send(sb.String())
}

func (b *TelegramBot) handleStatus(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Укажите ID встречи: /status <meeting_id>")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	meeting, err := b.client.Status(userID, args[0])
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка получения статуса: %v", err))
	}
	var response string
	if meeting.Name != nil {
		response = fmt.Sprintf("ID: %s\nНазвание: %s\nСтатус: %s\nСоздана: %s\nОбновлена: %s", meeting.ID, *meeting.Name, meeting.Status, meeting.CreatedAt.Format(time.RFC3339), meeting.UpdatedAt.Format(time.RFC3339))
	} else {
		response = fmt.Sprintf("ID: %s\nСтатус: %s\nСоздана: %s\nОбновлена: %s", meeting.ID, meeting.Status, meeting.CreatedAt.Format(time.RFC3339), meeting.UpdatedAt.Format(time.RFC3339))
	}
	if meeting.Status == string(model.TaskStatusFailed) && meeting.ProcessErrorMessage != nil {
		response = fmt.Sprintf("%s\nПричина ошибки: %s", response, *meeting.ProcessErrorMessage)
	}
	return c.Send(response)
}

func (b *TelegramBot) handleTranscription(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Укажите ID встречи: /transcription <meeting_id>")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	resp, err := b.client.Transcription(userID, args[0])
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка получения транскрипции: %v", err))
	}
	return c.Send(resp.Text)
}

func (b *TelegramBot) handleRetry(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Укажите ID встречи: /retry <meeting_id>")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	if err := b.client.Retry(userID, args[0]); err != nil {
		return c.Send(fmt.Sprintf("Ошибка повторной обработки встречи: %v", err))
	}
	return c.Send("Встреча поставлена в очередь на повторную обработку.")
}

func (b *TelegramBot) handleDelete(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Укажите ID встречи: /delete <meeting_id>")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	if err := b.client.Delete(userID, args[0]); err != nil {
		return c.Send(fmt.Sprintf("Ошибка удаления встречи: %v", err))
	}
	return c.Send("Встреча успешно удалена.")
}

func (b *TelegramBot) handleFind(c tele.Context) error {
	query := strings.TrimSpace(c.Message().Payload)
	if query == "" {
		return c.Send("Укажите поисковый запрос: /find <ключевые слова>")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	meetings, err := b.client.Find(userID, query)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка поиска: %v", err))
	}
	if len(meetings) == 0 {
		return c.Send("Ничего не найдено.")
	}

	var sb strings.Builder
	sb.WriteString("Найденные встречи:\n\n")
	for _, m := range meetings {
		if m.Name != nil {
			sb.WriteString(fmt.Sprintf("ID: %s\nНазвание: %s\nСтатус: %s\n", m.ID, *m.Name, m.Status))
		} else {
			sb.WriteString(fmt.Sprintf("ID: %s\nСтатус: %s\n", m.ID, m.Status))
		}
		if m.Summary != nil {
			sb.WriteString(fmt.Sprintf("Выжимка: %s\n", *m.Summary))
		}
		sb.WriteString("\n")
	}
	return c.Send(sb.String())
}

func (b *TelegramBot) handleChat(c tele.Context) error {
	question := strings.TrimSpace(c.Message().Text)
	if question == "" {
		return c.Send("Задайте вопрос")
	}
	userID := UserIDFromTelegramID(c.Sender().ID)
	resp, err := b.client.Chat(userID, question)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка обращения к ассистенту: %v", err))
	}
	return c.Send(resp.Answer)
}
