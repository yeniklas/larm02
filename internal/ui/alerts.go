package ui

import (
	"fmt"
	"sort"
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

	var sb strings.Builder

	// header row
	var headerCells []string
	for _, col := range columns {
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

	// Calculate how many rows we can display (height - 1 for the header).
	maxRows := height - 1
	if maxRows < 1 {
		maxRows = 1
	}

	// Scroll window: keep cursor visible.
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
		case displayItemGroup:
			line = renderGroupHeader(item.group, width)
			if i == cursor {
				line = styleSelected.Width(width).Render(line)
			}
		case displayItemAlert:
			row := formatRow(item.alert, width, extraCols)
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

func renderGroupHeader(g alertmanager.AlertGroup, width int) string {
	keys := make([]string, 0, len(g.Labels))
	for k := range g.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+g.Labels[k])
	}
	label := strings.Join(parts, "  ")

	const prefix = "─── "
	const suffix = " "
	// 2 for left indent
	remaining := width - 2 - len(prefix) - len(label) - len(suffix)
	if remaining < 2 {
		remaining = 2
	}
	line := "  " + prefix + label + suffix + strings.Repeat("─", remaining)
	return lipgloss.NewStyle().Foreground(colorAccent).Render(line)
}

func formatRow(a alertmanager.Alert, _ int, extraCols []config.ColumnConfig) string {
	alertname := truncate(a.Labels["alertname"], columns[colAlertname].width)
	severity := a.Labels["severity"]
	instance := truncate(a.Instance, columns[colInstance].width)
	state := a.Status.State
	started := humanDuration(time.Since(a.StartsAt))

	sev := severityStyle(severity).Width(columns[colSeverity].width).Render(truncate(severity, columns[colSeverity].width))
	st := stateStyle(state).Width(columns[colState].width).Render(truncate(state, columns[colState].width))

	cells := []string{
		lipgloss.NewStyle().Width(columns[colAlertname].width).Render(alertname),
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
