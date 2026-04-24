package tui

import "github.com/charmbracelet/lipgloss"

const (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#06B6D4")
	successColor   = lipgloss.Color("#10B981")
	warningColor   = lipgloss.Color("#F59E0B")
	errorColor     = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")
	textColor      = lipgloss.Color("#F9FAFB")
)

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	sourceStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	completedStyle = lipgloss.NewStyle().
			Foreground(successColor)

	downloadingStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	failedStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	pausedStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	pendingStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)
)
