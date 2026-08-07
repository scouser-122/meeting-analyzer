package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenLoad
	ScreenList
	ScreenStatus
	ScreenTranscription
	ScreenFind
	ScreenChat
	ScreenResult
)

type MenuItem struct {
	Label       string
	Description string
	Screen      Screen
}

var MenuItems = []MenuItem{
	{Label: "Загрузить", Description: "Загрузка аудио-файла с записью встречи", Screen: ScreenLoad},
	{Label: "Список встреч", Description: "Получить список встреч пользователя", Screen: ScreenList},
	{Label: "Статус по встрече", Description: "Получить статус обработки встречи по ID", Screen: ScreenStatus},
	{Label: "Транскрипция встречи", Description: "Получить транскрипцию встречи по ID", Screen: ScreenTranscription},
	{Label: "Поиск по фразе", Description: "Поиск встреч по ключевым словам", Screen: ScreenFind},
	{Label: "Чат", Description: "Задайте вопрос по теме загруженных встреч", Screen: ScreenChat},
}

type Model struct {
	API    *TuiClient
	UserID string

	// Navigation
	Screen     Screen
	MenuCursor int

	// Forms
	FilePathInput    textinput.Model
	MeetingNameInput textinput.Model
	MeetingIDInput   textinput.Model
	KeywordsInput    textinput.Model
	QuestionInput    textinput.Model
	FocusedInput     int

	// Results
	Result      string
	ResultTitle string
	Error       string
	Loading     bool

	// Window size
	Width  int
	Height int
}

func NewModel(apiClient *TuiClient, userID string) Model {
	fp := textinput.New()
	fp.Placeholder = "/path/to/audio/file.mp3"
	fp.Width = 120

	mn := textinput.New()
	mn.Placeholder = "Ведите имя встречи"
	mn.Width = 120

	mi := textinput.New()
	mi.Placeholder = "Введите ID встречи"
	mi.Width = 120

	kw := textinput.New()
	kw.Placeholder = "Ведите ключевую фразу для поиска"
	kw.Width = 120

	q := textinput.New()
	q.Placeholder = "Задайте вопрос по загруженным встречам"
	q.Width = 120

	return Model{
		API:              apiClient,
		UserID:           userID,
		Screen:           ScreenMainMenu,
		FilePathInput:    fp,
		MeetingNameInput: mn,
		MeetingIDInput:   mi,
		KeywordsInput:    kw,
		QuestionInput:    q,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func styleMeetingStatus(status string) string {
	if s, ok := meetingStatusStyle[status]; ok {
		return s.Render(status)
	}
	return status
}

func formatMeeting(m models.MeetingResponseData) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ID: %s\n", m.ID))
	sb.WriteString(fmt.Sprintf("Name: %s\n", *m.Name))
	sb.WriteString(fmt.Sprintf("Created: %s\n", m.CreatedAt))
	sb.WriteString(fmt.Sprintf("Status: %s\n", styleMeetingStatus(m.Status)))
	if m.Summary != nil {
		sb.WriteString(fmt.Sprintf("Summary: %s\n", *m.Summary))
	}
	return sb.String()
}
