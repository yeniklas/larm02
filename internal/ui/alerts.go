package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/larm02/internal/alertmanager"
	"github.com/yeniklas/larm02/internal/config"
)

const (
	colAlertname = 0
	colSeverity  = 1
	colInstance  = 2
	colState     = 3
	colStarted   = 4
)

var columns = []struct {
	header string
	width  int
}{
	{"ALERTNAME", 28},
	{"SEVERITY", 12},
	{"INSTANCE", 16},
	{"STATE", 12},
	{"STARTED", 12},
}

func renderAlertsTable(items []displayItem, cursor, width, height int, loading bool, sp spinner.Model, extraCols []config.ColumnConfig) string {
	if loading && len(items) == 0 {
		pad := height / 2
		return strings.Repeat("\n", pad) + "  " + sp.View() + " Loading alerts…"
	}

	// Compute alertname column width from actual content.
	nameWidth := len("ALERTNAME")
	for _, item := range items {
		var n string
		switch item.kind {
		case displayItemGroup:
			n = fmt.Sprintf("%s (%d)", item.group.Labels["alertname"], len(item.alerts))
		case displayItemAlert:
			n = item.alert.Labels["alertname"]
		}
		if len(n) > nameWidth {
			nameWidth = len(n)
		}
	}

	var sb strings.Builder

	// header row
	var headerCells []string
	headerCells = append(headerCells, styleHeaderCell.Width(nameWidth).Render(columns[colAlertname].header))
	for _, col := range columns[1:] {
		headerCells = append(headerCells, styleHeaderCell.Width(col.width).Render(col.header))
	}
	for _, col := range extraCols {
		headerCells = append(headerCells, styleHeaderCell.Width(col.GetWidth()).Render(col.GetHeader()))
	}
	sb.WriteString("  " + strings.Join(headerCells, " ") + "\n")

	if len(items) == 0 {
		sb.WriteString("\n  " + styleRefresh.Render("No alerts."))
		return sb.String()
	}

	maxRows := height - 1
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		var line string
		switch item.kind {
		case displayItemSection:
			line = renderSectionHeader(item, width)
			if i == cursor {
				line = styleSelected.Width(width).Render(line)
			}
		case displayItemGroup:
			row := formatGroupRow(item.group, item.alerts, nameWidth, extraCols)
			line = "  " + row
			if i == cursor {
				line = styleSelected.Width(width).Render(line)
				line = strings.Replace(line, "  ", " ▶", 1)
			}
		case displayItemAlert:
			row := formatRow(item.alert, nameWidth, extraCols)
			line = "  " + row
			if i == cursor {
				line = styleSelected.Width(width).Render(line)
				line = strings.Replace(line, "  ", " ▶", 1)
			}
		}
		sb.WriteString(line + "\n")
	}

	if len(items) > maxRows {
		sb.WriteString(styleRefresh.Render(fmt.Sprintf("  %d-%d of %d", start+1, end, len(items))) + "\n")
	}

	return sb.String()
}

func formatGroupRow(g alertmanager.AlertGroup, alerts []alertmanager.Alert, nameWidth int, extraCols []config.ColumnConfig) string {
	name := g.Labels["alertname"]
	alertname := truncate(fmt.Sprintf("%s (%d)", name, len(alerts)), nameWidth)
	severity := maxSeverity(alerts)
	instance := groupInstance(alerts)
	state := dominantState(alerts)
	started := humanDuration(time.Since(oldestStart(alerts)))

	sev := severityStyle(severity).Width(columns[colSeverity].width).Render(truncate(severity, columns[colSeverity].width))
	st := stateStyle(state).Width(columns[colState].width).Render(truncate(state, columns[colState].width))

	cells := []string{
		lipgloss.NewStyle().Width(nameWidth).Render(alertname),
		sev,
		lipgloss.NewStyle().Width(columns[colInstance].width).Render(truncate(instance, columns[colInstance].width)),
		st,
		lipgloss.NewStyle().Width(columns[colStarted].width).Render(started),
	}
	for _, col := range extraCols {
		val := truncate(g.Labels[col.Label], col.GetWidth())
		cells = append(cells, lipgloss.NewStyle().Width(col.GetWidth()).Render(val))
	}
	return strings.Join(cells, " ")
}

func formatRow(a alertmanager.Alert, nameWidth int, extraCols []config.ColumnConfig) string {
	alertname := truncate(a.Labels["alertname"], nameWidth)
	severity := a.Labels["severity"]
	instance := truncate(a.Instance, columns[colInstance].width)
	state := a.Status.State
	started := humanDuration(time.Since(a.StartsAt))

	sev := severityStyle(severity).Width(columns[colSeverity].width).Render(truncate(severity, columns[colSeverity].width))
	st := stateStyle(state).Width(columns[colState].width).Render(truncate(state, columns[colState].width))

	cells := []string{
		lipgloss.NewStyle().Width(nameWidth).Render(alertname),
		sev,
		lipgloss.NewStyle().Width(columns[colInstance].width).Render(instance),
		st,
		lipgloss.NewStyle().Width(columns[colStarted].width).Render(started),
	}
	for _, col := range extraCols {
		val := truncate(a.Labels[col.Label], col.GetWidth())
		cells = append(cells, lipgloss.NewStyle().Width(col.GetWidth()).Render(val))
	}
	return strings.Join(cells, " ")
}

func renderSectionHeader(item displayItem, width int) string {
	arrow := "▼"
	if item.collapsed {
		arrow = "▶"
	}
	noun := "groups"
	if item.groupCount == 1 {
		noun = "group"
	}
	label := fmt.Sprintf("%s=%s", item.sectionLabel, item.sectionValue)
	mid := fmt.Sprintf(" %s  %s  (%d %s) ", arrow, label, item.groupCount, noun)
	fillLen := width - len(mid) - 2
	if fillLen < 0 {
		fillLen = 0
	}
	half := fillLen / 2
	line := "  " + strings.Repeat("═", half) + mid + strings.Repeat("═", fillLen-half)
	return lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(line)
}

func maxSeverity(alerts []alertmanager.Alert) string {
	order := map[string]int{"critical": 4, "warning": 3, "info": 2, "informational": 2}
	best := ""
	bestRank := -1
	for _, a := range alerts {
		sev := a.Labels["severity"]
		if rank := order[sev]; rank > bestRank {
			best = sev
			bestRank = rank
		}
	}
	if best == "" && len(alerts) > 0 {
		best = alerts[0].Labels["severity"]
	}
	return best
}

func dominantState(alerts []alertmanager.Alert) string {
	for _, a := range alerts {
		if a.Status.State == "active" {
			return "active"
		}
	}
	for _, a := range alerts {
		if a.Status.State != "suppressed" {
			return a.Status.State
		}
	}
	if len(alerts) > 0 {
		return alerts[0].Status.State
	}
	return ""
}

func oldestStart(alerts []alertmanager.Alert) time.Time {
	if len(alerts) == 0 {
		return time.Time{}
	}
	t := alerts[0].StartsAt
	for _, a := range alerts[1:] {
		if a.StartsAt.Before(t) {
			t = a.StartsAt
		}
	}
	return t
}

func groupInstance(alerts []alertmanager.Alert) string {
	if len(alerts) == 0 {
		return ""
	}
	first := alerts[0].Instance
	unique := map[string]struct{}{first: {}}
	for _, a := range alerts[1:] {
		unique[a.Instance] = struct{}{}
	}
	if len(unique) == 1 {
		return first
	}
	return fmt.Sprintf("%d instances", len(unique))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
