package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var helpBindings = [][2]string{
	{"j / ↓", "move down"},
	{"k / ↑", "move up"},
	{"Enter", "alert detail"},
	{"/", "filter alerts"},
	{"ESC", "cancel / close"},
	{":", "command mode"},
	{"r", "refresh now"},
	{"?", "toggle help"},
	{"q", "quit"},
}

func renderHelp(width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 3)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Key Bindings") + "\n\n")
	for _, b := range helpBindings {
		sb.WriteString(keyStyle.Render(b[0]) + "  " + descStyle.Render(b[1]) + "\n")
	}

	box := boxStyle.Render(sb.String())

	// centre horizontally
	boxWidth := lipgloss.Width(box)
	pad := (width - boxWidth) / 2
	if pad < 0 {
		pad = 0
	}
	return lipgloss.NewStyle().MarginLeft(pad).Render(box)
}
