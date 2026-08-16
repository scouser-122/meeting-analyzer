package tui

import (
	"fmt"
	"strings"
)

// View renders the current TUI screen.
func (m Model) View() string {
	switch m.Screen {
	case ScreenMainMenu:
		return m.viewMainMenu()
	case ScreenLoad:
		return m.viewLoadForm()
	case ScreenList:
		return m.viewLoading("Получение встреч...")
	case ScreenStatus:
		return m.viewStatusForm()
	case ScreenTranscription:
		return m.viewTranscriptionForm()
	case ScreenDelete:
		return m.viewDeleteForm()
	case ScreenFind:
		return m.viewFindForm()
	case ScreenChat:
		return m.viewChatForm()
	case ScreenResult:
		return m.viewResult()
	default:
		return ""
	}
}

func (m Model) viewMainMenu() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Meeting Analyzer CLI"))
	sb.WriteString("\n")
	sb.WriteString(userIDStyle.Render(fmt.Sprintf("ID пользователя: %s", m.UserID)))
	sb.WriteString("\n")
	sb.WriteString(subtitleStyle.Render("Выберите команду:"))
	sb.WriteString("\n\n")

	for i, item := range MenuItems {
		if i == m.MenuCursor {
			sb.WriteString(selectedItemStyle.Render(fmt.Sprintf("▶ %s", item.Label)))
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("   %s\n", subtitleStyle.Render(item.Description)))
		} else {
			sb.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s", item.Label)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString(helpStyle.Render("\n↑/↓ navigate • enter select • q quit"))

	return sb.String()
}

func (m Model) viewLoadForm() string {
	if m.Loading {
		return m.viewLoading("Загрузка файла встречи...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Загрузка Записи Встречи"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("Путь к файлу:"))
	sb.WriteString("\n")
	sb.WriteString(m.FilePathInput.View())
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("Имя встречи:"))
	sb.WriteString("\n")
	sb.WriteString(m.MeetingNameInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewStatusForm() string {
	if m.Loading {
		return m.viewLoading("Получение статуса обработки встречи...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Получение Статуса Встречи"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("ID встречи:"))
	sb.WriteString("\n")
	sb.WriteString(m.MeetingIDInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewTranscriptionForm() string {
	if m.Loading {
		return m.viewLoading("Получение транскрипции по встрече...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Получение Транскрипции по Встрече"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("ID встречи:"))
	sb.WriteString("\n")
	sb.WriteString(m.MeetingIDInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewDeleteForm() string {
	if m.Loading {
		return m.viewLoading("Удаление встречи...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Удаление Встречи"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("ID встречи:"))
	sb.WriteString("\n")
	sb.WriteString(m.MeetingIDInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewFindForm() string {
	if m.Loading {
		return m.viewLoading("Поиск встреч...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Поиск Встреч"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("Ключевая фраза:"))
	sb.WriteString("\n")
	sb.WriteString(m.KeywordsInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewChatForm() string {
	if m.Loading {
		return m.viewLoading("Задаём вопрос...")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Чат по теме загруженных встреч"))
	sb.WriteString("\n\n")
	sb.WriteString(inputLabelStyle.Render("Вопрос:"))
	sb.WriteString("\n")
	sb.WriteString(m.QuestionInput.View())
	sb.WriteString(m.viewFormHelp())

	if m.Error != "" {
		sb.WriteString("\n\n")
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %s", m.Error)))
	}

	return sb.String()
}

func (m Model) viewResult() string {
	if m.Loading {
		return m.viewLoading("Обработка...")
	}

	var sb strings.Builder

	if m.Error != "" {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("✗ %s", m.Error)))
		sb.WriteString("\n")
	} else {
		if m.ResultTitle != "" {
			sb.WriteString(successStyle.Render(fmt.Sprintf("✓ %s", m.ResultTitle)))
			sb.WriteString("\n\n")
		}
		sb.WriteString(resultStyle.Render(m.Result))
	}

	sb.WriteString(helpStyle.Render("\n\nНажмите на любую клавишу для возврата в меню"))

	return sb.String()
}

func (m Model) viewLoading(message string) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(message))
	sb.WriteString("\n\n")
	sb.WriteString(spinnerStyle.Render("⏳ Пожалуйста подождите..."))
	return sb.String()
}

func (m Model) viewFormHelp() string {
	return helpStyle.Render("\n\nesc назад • enter отправить • tab перейти к следующему пункту")
}
