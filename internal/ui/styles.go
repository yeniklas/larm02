package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorCritical = lipgloss.Color("196") // bright red
	colorWarning  = lipgloss.Color("214") // orange/yellow
	colorInfo     = lipgloss.Color("39")  // blue
	colorGood     = lipgloss.Color("46")  // green
	colorMuted    = lipgloss.Color("240") // gray
	colorAccent   = lipgloss.Color("205") // pink/magenta accent

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("17")).
			PaddingLeft(1).PaddingRight(1)

	styleInstance = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("27")).
			PaddingLeft(1).PaddingRight(1).
			MarginLeft(1)

	styleRefresh = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(1)

	styleFooterKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true)

	styleBreadcrumb = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			PaddingLeft(1)

	styleFilter = lipgloss.NewStyle().
			Foreground(colorAccent).
			Italic(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorCritical)

	styleDetailKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Width(20)

	styleDetailVal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	styleSectionHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent).
				MarginTop(1)
)

func severityStyle(severity string) lipgloss.Style {
	switch severity {
	case "critical":
		return lipgloss.NewStyle().Foreground(colorCritical).Bold(true)
	case "warning":
		return lipgloss.NewStyle().Foreground(colorWarning)
	case "info", "informational":
		return lipgloss.NewStyle().Foreground(colorInfo)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	}
}

func stateStyle(state string) lipgloss.Style {
	switch state {
	case "active":
		return lipgloss.NewStyle().Foreground(colorGood)
	case "suppressed":
		return lipgloss.NewStyle().Foreground(colorMuted)
	default:
		return lipgloss.NewStyle().Foreground(colorWarning)
	}
}
