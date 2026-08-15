package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type apiResultMsg struct {
	result string
	title  string
	err    error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case apiResultMsg:
		m.Loading = false
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Screen = ScreenResult
			m.ResultTitle = "Ошибка"
			return m, nil
		}
		m.Error = ""
		m.Result = msg.result
		m.ResultTitle = msg.title
		m.Screen = ScreenResult
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit from main menu
	if key == "ctrl+c" || key == "q" {
		if m.Screen == ScreenMainMenu {
			return m, tea.Quit
		}
	}

	if m.Loading {
		return m, nil
	}

	switch m.Screen {
	case ScreenMainMenu:
		return m.handleMainMenuKeys(key)
	case ScreenResult:
		return m.handleResultKeys(key)
	default:
		return m.handleFormKeys(msg, key)
	}
}

func (m Model) handleMainMenuKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.MenuCursor > 0 {
			m.MenuCursor--
		}
	case "down", "j":
		if m.MenuCursor < len(MenuItems)-1 {
			m.MenuCursor++
		}
	case "enter", " ":
		item := MenuItems[m.MenuCursor]
		switch item.Screen {
		case ScreenList:
			m.Loading = true
			m.Screen = ScreenList
			return m, m.fetchList()
		default:
			m.Screen = item.Screen
			m.FocusedInput = 0
			return m, m.focusFirstInput()
		}
	}
	return m, nil
}

func (m Model) handleResultKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter", "backspace":
		m.Screen = ScreenMainMenu
		m.Result = ""
		m.ResultTitle = ""
		m.Error = ""
	}
	return m, nil
}

func (m Model) handleFormKeys(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	// First, let textinputs process the keystroke
	updated, cmd := m.updateInputs(msg)
	m = updated

	switch key {
	case "esc":
		m.Screen = ScreenMainMenu
		m.clearInputs()
		return m, nil

	case "tab":
		cmd = m.cycleFocus()
		return m, cmd

	case "enter":
		return m.submitForm()
	}

	return m, cmd
}

func (m *Model) focusFirstInput() tea.Cmd {
	switch m.Screen {
	case ScreenLoad:
		return m.FilePathInput.Focus()
	case ScreenStatus, ScreenTranscription:
		return m.MeetingIDInput.Focus()
	case ScreenFind:
		return m.KeywordsInput.Focus()
	case ScreenChat:
		return m.QuestionInput.Focus()
	}
	return nil
}

func (m *Model) cycleFocus() tea.Cmd {
	switch m.Screen {
	case ScreenLoad:
		m.FocusedInput = (m.FocusedInput + 1) % 2
		if m.FocusedInput == 0 {
			m.MeetingNameInput.Blur()
			return m.FilePathInput.Focus()
		}
		m.FilePathInput.Blur()
		return m.MeetingNameInput.Focus()

	case ScreenStatus, ScreenTranscription:
		return nil

	case ScreenFind:
		return nil

	case ScreenChat:
		return nil
	}
	return nil
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenLoad:
		filePath := strings.TrimSpace(m.FilePathInput.Value())
		meetingName := strings.TrimSpace(m.MeetingNameInput.Value())
		if filePath == "" || meetingName == "" {
			m.Error = "Не указан путь к файлу или имя встречи"
			return m, nil
		}
		m.Loading = true
		m.FilePathInput.Blur()
		m.MeetingNameInput.Blur()
		return m, m.loadMeeting(filePath, meetingName)

	case ScreenStatus:
		meetingID := strings.TrimSpace(m.MeetingIDInput.Value())
		if meetingID == "" {
			m.Error = "Не указан ID встречи"
			return m, nil
		}
		m.Loading = true
		m.MeetingIDInput.Blur()
		return m, m.fetchStatus(meetingID)

	case ScreenTranscription:
		meetingID := strings.TrimSpace(m.MeetingIDInput.Value())
		if meetingID == "" {
			m.Error = "Не указан ID встречи"
			return m, nil
		}
		m.Loading = true
		m.MeetingIDInput.Blur()
		return m, m.fetchTranscription(meetingID)

	case ScreenFind:
		keywords := strings.TrimSpace(m.KeywordsInput.Value())
		if keywords == "" {
			m.Error = "Не указана ключевая фраза"
			return m, nil
		}
		m.Loading = true
		m.KeywordsInput.Blur()
		return m, m.fetchFind(keywords)

	case ScreenChat:
		question := strings.TrimSpace(m.QuestionInput.Value())
		if question == "" {
			m.Error = "Не указан вопрос"
			return m, nil
		}
		m.Loading = true
		m.QuestionInput.Blur()
		return m, m.fetchChat(question)
	}
	return m, nil
}

func (m Model) updateInputs(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m.Screen {
	case ScreenLoad:
		if m.FocusedInput == 0 {
			var cmd tea.Cmd
			m.FilePathInput, cmd = m.FilePathInput.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			m.MeetingNameInput, cmd = m.MeetingNameInput.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ScreenStatus, ScreenTranscription:
		var cmd tea.Cmd
		m.MeetingIDInput, cmd = m.MeetingIDInput.Update(msg)
		cmds = append(cmds, cmd)

	case ScreenFind:
		var cmd tea.Cmd
		m.KeywordsInput, cmd = m.KeywordsInput.Update(msg)
		cmds = append(cmds, cmd)

	case ScreenChat:
		var cmd tea.Cmd
		m.QuestionInput, cmd = m.QuestionInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m *Model) clearInputs() {
	m.FilePathInput.Reset()
	m.MeetingNameInput.Reset()
	m.MeetingIDInput.Reset()
	m.KeywordsInput.Reset()
	m.QuestionInput.Reset()
	m.Error = ""
	m.FocusedInput = 0
}

func (m Model) fetchList() tea.Cmd {
	return func() tea.Msg {
		meetings, err := m.API.List(m.UserID)
		if err != nil {
			return apiResultMsg{err: err}
		}

		var sb strings.Builder
		if len(meetings) == 0 {
			sb.WriteString("Встречи не найдены.")
		} else {
			for i, meeting := range meetings {
				if i > 0 {
					sb.WriteString(strings.Repeat("─", 60) + "\n")
				}
				sb.WriteString(formatMeeting(meeting))
				sb.WriteString("\n")
			}
		}

		return apiResultMsg{
			result: sb.String(),
			title:  fmt.Sprintf("Встречи по %s", m.UserID),
		}
	}
}

func (m Model) loadMeeting(filePath, meetingName string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.API.Load(m.UserID, meetingName, filePath)
		if err != nil {
			return apiResultMsg{err: err}
		}

		result := fmt.Sprintf("Встреча успешно загружена!\n\nID: %s\nИмя: %s",
			resp.Data.ID, *resp.Data.MeetingName)
		return apiResultMsg{result: result, title: "Результат загрузки"}
	}
}

func (m Model) fetchStatus(meetingID string) tea.Cmd {
	return func() tea.Msg {
		meeting, err := m.API.Status(m.UserID, meetingID)
		if err != nil {
			return apiResultMsg{err: err}
		}

		return apiResultMsg{
			result: formatMeeting(*meeting),
			title:  "Статус по встрече",
		}
	}
}

func (m Model) fetchTranscription(meetingID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.API.Transcription(m.UserID, meetingID)
		if err != nil {
			return apiResultMsg{err: err}
		}

		return apiResultMsg{
			result: resp.Text,
			title:  "Transcription",
		}
	}
}

func (m Model) fetchFind(keywords string) tea.Cmd {
	return func() tea.Msg {
		meetings, err := m.API.Find(m.UserID, keywords)
		if err != nil {
			return apiResultMsg{err: err}
		}

		var sb strings.Builder
		if len(meetings) == 0 {
			sb.WriteString("Не найдены встречи по заданной ключевой фразе.")
		} else {
			for i, meeting := range meetings {
				if i > 0 {
					sb.WriteString(strings.Repeat("─", 60) + "\n")
				}
				sb.WriteString(formatMeeting(meeting))
				sb.WriteString("\n")
			}
		}

		return apiResultMsg{
			result: sb.String(),
			title:  fmt.Sprintf("Результаты поиска для: %s", keywords),
		}
	}
}

func (m Model) fetchChat(question string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.API.Chat(m.UserID, question)
		if err != nil {
			return apiResultMsg{err: err}
		}

		return apiResultMsg{
			result: resp.Answer,
			title:  "Ответ",
		}
	}
}
