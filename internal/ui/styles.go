package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/larm02/internal/config"
)

var (
	colorCritical lipgloss.Color
	colorWarning  lipgloss.Color
	colorInfo     lipgloss.Color
	colorGood     lipgloss.Color
	colorMuted    lipgloss.Color
	colorAccent   lipgloss.Color

	styleHeader        lipgloss.Style
	styleInstance      lipgloss.Style
	styleRefresh       lipgloss.Style
	styleFooter        lipgloss.Style
	styleFooterKey     lipgloss.Style
	styleBreadcrumb    lipgloss.Style
	styleFilter        lipgloss.Style
	styleError         lipgloss.Style
	styleDetailKey     lipgloss.Style
	styleDetailVal     lipgloss.Style
	styleSectionHeader lipgloss.Style
	styleHeaderCell    lipgloss.Style
	styleSelected      lipgloss.Style
)

func init() {
	ApplyTheme(config.DefaultTheme())
}

// ApplyTheme rebuilds all package-level styles from the given theme.
// Call this once before starting the TUI program.
func ApplyTheme(t config.Theme) {
	colorCritical = lipgloss.Color(t.Critical)
	colorWarning = lipgloss.Color(t.Warning)
	colorInfo = lipgloss.Color(t.Info)
	colorGood = lipgloss.Color(t.Good)
	colorMuted = lipgloss.Color(t.Muted)
	colorAccent = lipgloss.Color(t.Accent)

	fg := lipgloss.Color(t.HeaderFg)
	hdrBg := lipgloss.Color(t.HeaderBg)
	instBg := lipgloss.Color(t.InstanceBg)
	selBg := lipgloss.Color(t.SelectedBg)

	styleHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(fg).
		Background(hdrBg).
		PaddingLeft(1).PaddingRight(1)

	styleInstance = lipgloss.NewStyle().
		Foreground(fg).
		Background(instBg).
		PaddingLeft(1).PaddingRight(1).
		MarginLeft(1)

	styleRefresh = lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true)

	styleFooter = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	styleFooterKey = lipgloss.NewStyle().
		Foreground(fg).
		Bold(true)

	styleBreadcrumb = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	styleFilter = lipgloss.NewStyle().
		Foreground(colorAccent).
		Italic(true)

	styleError = lipgloss.NewStyle().
		Foreground(colorCritical)

	styleDetailKey = lipgloss.NewStyle().
		Foreground(colorMuted).
		Width(20)

	styleDetailVal = lipgloss.NewStyle().
		Foreground(fg)

	styleSectionHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAccent).
		MarginTop(1)

	styleHeaderCell = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMuted).
		Underline(true)

	styleSelected = lipgloss.NewStyle().
		Background(selBg)
}

func severityStyle(severity string) lipgloss.Style {
	switch severity {
	case "critical":
		return lipgloss.NewStyle().Foreground(colorCritical).Bold(true)
	case "warning":
		return lipgloss.NewStyle().Foreground(colorWarning)
	case "info", "informational":
		return lipgloss.NewStyle().Foreground(colorInfo)
	default:
		return lipgloss.NewStyle().Foreground(colorGood)
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
