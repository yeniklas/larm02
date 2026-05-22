package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/larm02/internal/alertmanager"
)

func renderDetail(a alertmanager.Alert, width int) string {
	var sb strings.Builder

	header := fmt.Sprintf("Alert Detail — %s", a.Labels["alertname"])
	sb.WriteString(styleSectionHeader.Render(header) + "\n\n")

	sb.WriteString(styleSectionHeader.Render("Labels") + "\n")
	keys := sortedKeys(a.Labels)
	for _, k := range keys {
		sb.WriteString(styleDetailKey.Render(k) + styleDetailVal.Render(a.Labels[k]) + "\n")
	}

	if len(a.Annotations) > 0 {
		sb.WriteString("\n" + styleSectionHeader.Render("Annotations") + "\n")
		keys = sortedKeys(a.Annotations)
		for _, k := range keys {
			val := wordWrap(a.Annotations[k], width-22)
			sb.WriteString(styleDetailKey.Render(k) + styleDetailVal.Render(val) + "\n")
		}
	}

	sb.WriteString("\n" + styleSectionHeader.Render("Status") + "\n")
	sb.WriteString(styleDetailKey.Render("state") + stateStyle(a.Status.State).Render(a.Status.State) + "\n")
	sb.WriteString(styleDetailKey.Render("instance") + styleDetailVal.Render(a.Instance) + "\n")
	sb.WriteString(styleDetailKey.Render("started") + styleDetailVal.Render(humanDuration(time.Since(a.StartsAt))+" ago") + "\n")
	if len(a.Status.SilencedBy) > 0 {
		sb.WriteString(styleDetailKey.Render("silenced by") + styleDetailVal.Render(strings.Join(a.Status.SilencedBy, ", ")) + "\n")
	}
	if len(a.Status.InhibitedBy) > 0 {
		sb.WriteString(styleDetailKey.Render("inhibited by") + styleDetailVal.Render(strings.Join(a.Status.InhibitedBy, ", ")) + "\n")
	}
	if a.GeneratorURL != "" {
		sb.WriteString(styleDetailKey.Render("source") + styleDetailVal.Render(a.GeneratorURL) + "\n")
	}

	sb.WriteString("\n" + styleFooter.Render("<ESC> back"))

	return sb.String()
}

func renderGroupDetail(g alertmanager.AlertGroup, width int) string {
	var sb strings.Builder

	keys := sortedKeys(g.Labels)
	labelParts := make([]string, 0, len(keys))
	for _, k := range keys {
		labelParts = append(labelParts, k+"="+g.Labels[k])
	}
	header := fmt.Sprintf("Group — %s", strings.Join(labelParts, "  "))
	sb.WriteString(styleSectionHeader.Render(header) + "\n\n")

	if g.Receiver.Name != "" {
		sb.WriteString(styleDetailKey.Render("receiver") + styleDetailVal.Render(g.Receiver.Name) + "\n\n")
	}

	sb.WriteString(styleSectionHeader.Render(fmt.Sprintf("Alerts (%d)", len(g.Alerts))) + "\n")
	for _, a := range g.Alerts {
		alertname := truncate(a.Labels["alertname"], columns[colAlertname].width)
		severity := a.Labels["severity"]
		instance := truncate(a.Instance, columns[colInstance].width)
		state := a.Status.State
		since := humanDuration(time.Since(a.StartsAt))

		line := fmt.Sprintf("  %s  %s  %s  %s  %s ago",
			lipgloss.NewStyle().Width(columns[colAlertname].width).Render(alertname),
			severityStyle(severity).Width(columns[colSeverity].width).Render(truncate(severity, columns[colSeverity].width)),
			lipgloss.NewStyle().Width(columns[colInstance].width).Render(instance),
			stateStyle(state).Width(columns[colState].width).Render(truncate(state, columns[colState].width)),
			since,
		)
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n" + styleFooter.Render("<ESC> back"))
	return sb.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func wordWrap(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	var out strings.Builder
	for i, r := range s {
		if i > 0 && i%width == 0 {
			out.WriteRune('\n')
			out.WriteString(strings.Repeat(" ", 22)) // align with value column
		}
		out.WriteRune(r)
	}
	return out.String()
}
