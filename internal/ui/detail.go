package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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
