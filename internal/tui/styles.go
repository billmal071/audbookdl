package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

	// Card-like container for search results
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1).
			MarginBottom(0)

	// Selected card has highlighted border
	selectedCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1).
				MarginBottom(0)

	// Source badge styles — colored pills
	librivoxBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#059669")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	archiveBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#2563EB")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	loyalbooksBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#D97706")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	openlibraryBadge = lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Bold(true)

	// Duration/chapter count tag
	tagStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// Divider
	dividerStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Search input frame
	inputFrameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	// Section header
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				MarginTop(1).
				MarginBottom(1)

	// Detail panel
	detailPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2)

	// Progress bar colors
	progressFullStyle  = lipgloss.NewStyle().Foreground(successColor)
	progressEmptyStyle = lipgloss.NewStyle().Foreground(mutedColor)
)

// sourceBadge returns a styled badge for the given source name.
func sourceBadge(source string) string {
	switch source {
	case "librivox":
		return librivoxBadge.Render(" LV ")
	case "archive":
		return archiveBadge.Render(" IA ")
	case "loyalbooks":
		return loyalbooksBadge.Render(" LB ")
	case "openlibrary":
		return openlibraryBadge.Render(" OL ")
	default:
		return subtitleStyle.Render(source)
	}
}

// styledProgressBar renders a progress bar using styled Unicode characters.
func styledProgressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	bar := progressFullStyle.Render(strings.Repeat("━", filled))
	bar += progressEmptyStyle.Render(strings.Repeat("─", empty))
	return bar
}
