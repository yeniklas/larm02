package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/larm02/internal/config"
)

func renderInstances(instances []config.AlertmanagerConfig, hidden map[string]bool, counts map[string]int, cursor int, width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginBottom(1)
	checkStyle := lipgloss.NewStyle().Foreground(colorGood).Bold(true)
	uncheckedStyle := lipgloss.NewStyle().Foreground(colorMuted)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Width(20)
	countStyle := lipgloss.NewStyle().Foreground(colorMuted)
	cursorStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 3)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Instance Visibility") + "\n\n")

	for i, am := range instances {
		cur := "  "
		if i == cursor {
			cur = cursorStyle.Render("▶ ")
		}

		var check string
		if hidden[am.Name] {
			check = uncheckedStyle.Render("[ ]")
		} else {
			check = checkStyle.Render("[x]")
		}

		name := nameStyle.Render(am.Name)
		count := countStyle.Render(fmt.Sprintf("(%d)", counts[am.Name]))
		sb.WriteString(cur + check + "  " + name + "  " + count + "\n")
	}

	sb.WriteString(hintStyle.Render("<Space> toggle  <j/k> move  <ESC> close"))

	box := boxStyle.Render(sb.String())

	boxWidth := lipgloss.Width(box)
	pad := (width - boxWidth) / 2
	if pad < 0 {
		pad = 0
	}
	return lipgloss.NewStyle().MarginLeft(pad).Render(box)
}
