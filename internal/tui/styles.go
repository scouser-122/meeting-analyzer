package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			MarginBottom(1)

	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#7C3AED")).
				Bold(true)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	resultStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(1).MarginTop(1)

	meetingStatusStyle = map[string]lipgloss.Style{
		"created":     lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")),
		"processing":  lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")),
		"transcribed": lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6")),
		"summarized":  lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4")),
		"completed":   lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")),
		"failed":      lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")),
	}

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED"))

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4B5563"))

	userIDStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)
