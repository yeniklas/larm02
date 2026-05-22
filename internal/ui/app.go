package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/larm02/internal/alertmanager"
	"github.com/yeniklas/larm02/internal/config"
)

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeCommand
	modeDetail
	modeHelp
)

type alertsFetchedMsg struct {
	alerts []alertmanager.Alert
	errs   []error
}

type tickMsg time.Time

// AppModel is the root bubbletea model.
type AppModel struct {
	cfg           *config.Config
	alerts        []alertmanager.Alert
	filtered      []alertmanager.Alert
	failingChecks []string // healthcheck names with no matching alerts
	cursor        int
	mode          mode
	filterInput   textinput.Model
	cmdInput      textinput.Model
	spinner       spinner.Model
	loading       bool
	lastRefresh   time.Time
	errs          []error
	width         int
	height        int
}

func New(cfg *config.Config) AppModel {
	fi := textinput.New()
	fi.Placeholder = "severity=critical"
	fi.CharLimit = 200

	ci := textinput.New()
	ci.Placeholder = "alerts, quit"
	ci.CharLimit = 100

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return AppModel{
		cfg:         cfg,
		filterInput: fi,
		cmdInput:    ci,
		spinner:     sp,
		loading:     true,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchAlerts(m.cfg),
		scheduleTick(m.cfg.GetRefreshInterval()),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tea.Batch(fetchAlerts(m.cfg), scheduleTick(m.cfg.GetRefreshInterval()))

	case alertsFetchedMsg:
		m.loading = false
		m.lastRefresh = time.Now()
		m.alerts, m.failingChecks = partitionHealthchecks(msg.alerts, m.cfg.Healthchecks)
		m.errs = msg.errs
		m.applyFilter()
		if m.cursor >= len(m.filtered) && m.cursor > 0 {
			m.cursor = len(m.filtered) - 1
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeCommand:
		return m.handleCommandKey(msg)
	case modeDetail:
		return m.handleDetailKey(msg)
	case modeHelp:
		if msg.Type == tea.KeyEsc || msg.String() == "?" || msg.String() == "q" {
			m.mode = modeNormal
		}
		return m, nil
	}
	return m.handleNormalKey(msg)
}

func (m AppModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.filtered) > 0 {
			m.mode = modeDetail
		}
	case "/":
		m.mode = modeFilter
		m.filterInput.Focus()
		return m, textinput.Blink
	case ":":
		m.mode = modeCommand
		m.cmdInput.SetValue("")
		m.cmdInput.Focus()
		return m, textinput.Blink
	case "r":
		m.loading = true
		return m, fetchAlerts(m.cfg)
	case "?":
		m.mode = modeHelp
	case "esc":
		m.filterInput.SetValue("")
		m.applyFilter()
		m.cursor = 0
	}
	return m, nil
}

func (m AppModel) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEsc:
		if msg.Type == tea.KeyEsc {
			m.filterInput.SetValue("")
		}
		m.filterInput.Blur()
		m.mode = modeNormal
		m.applyFilter()
		m.cursor = 0
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	m.cursor = 0
	return m, cmd
}

func (m AppModel) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		cmd := strings.TrimSpace(m.cmdInput.Value())
		m.cmdInput.SetValue("")
		m.cmdInput.Blur()
		m.mode = modeNormal
		switch cmd {
		case "quit", "q":
			return m, tea.Quit
		case "alerts":
			// already on alerts view
		}
		return m, nil
	case tea.KeyEsc:
		m.cmdInput.SetValue("")
		m.cmdInput.Blur()
		m.mode = modeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	return m, cmd
}

func (m AppModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc || msg.String() == "q" {
		m.mode = modeNormal
	}
	return m, nil
}

// applyFilter filters m.alerts into m.filtered based on filterInput value.
func (m *AppModel) applyFilter() {
	query := strings.TrimSpace(m.filterInput.Value())
	if query == "" {
		m.filtered = m.alerts
		return
	}

	var out []alertmanager.Alert
	for _, a := range m.alerts {
		if matchesFilter(a, query) {
			out = append(out, a)
		}
	}
	m.filtered = out
}

// matchesFilter checks if alert matches a simple filter expression.
// Supports "key=value", "key=~value" (substring), or plain substring of alertname.
func matchesFilter(a alertmanager.Alert, query string) bool {
	if strings.Contains(query, "=") {
		parts := strings.SplitN(query, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		substring := false
		if strings.HasPrefix(key, "~") {
			key = key[1:]
			substring = true
		}
		// support key=~value (regex-lite substring)
		if strings.HasPrefix(val, "~") {
			val = val[1:]
			substring = true
		}
		lv, ok := a.Labels[key]
		if !ok {
			lv = a.Annotations[key]
		}
		if substring {
			return strings.Contains(strings.ToLower(lv), strings.ToLower(val))
		}
		return strings.EqualFold(lv, val)
	}
	// plain substring — match alertname or instance
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(a.Labels["alertname"]), q) ||
		strings.Contains(strings.ToLower(a.Instance), q)
}

func (m AppModel) View() string {
	if m.mode == modeHelp {
		return m.renderHelp()
	}
	if m.mode == modeDetail && len(m.filtered) > 0 {
		return m.renderDetailView()
	}
	return m.renderMain()
}

func (m AppModel) renderMain() string {
	header := m.renderHeader()
	breadcrumb := m.renderBreadcrumb()
	footer := m.renderFooter()

	headerH := lipgloss.Height(header)
	breadcrumbH := lipgloss.Height(breadcrumb)
	footerH := lipgloss.Height(footer)
	tableH := m.height - headerH - breadcrumbH - footerH
	if tableH < 0 {
		tableH = 0
	}

	table := renderAlertsTable(m.filtered, m.cursor, m.width, tableH, m.loading, m.spinner)

	return lipgloss.JoinVertical(lipgloss.Left, header, breadcrumb, table, footer)
}

func (m AppModel) renderDetailView() string {
	header := m.renderHeader()
	detail := renderDetail(m.filtered[m.cursor], m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, detail)
}

func (m AppModel) renderHelp() string {
	header := m.renderHeader()
	help := renderHelp(m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, help)
}

func (m AppModel) renderHeader() string {
	left := styleHeader.Render("larm02")
	for _, am := range m.cfg.Alertmanagers {
		left += styleInstance.Render(am.Name)
	}

	var refreshStr string
	if m.loading {
		refreshStr = m.spinner.View() + " fetching…"
	} else if !m.lastRefresh.IsZero() {
		refreshStr = "Refreshed: " + humanDuration(time.Since(m.lastRefresh)) + " ago"
	}
	right := styleRefresh.Render(refreshStr)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m AppModel) renderBreadcrumb() string {
	crumb := "Alerts"
	filter := strings.TrimSpace(m.filterInput.Value())
	if filter != "" {
		crumb += "  " + styleFilter.Render("(filter: "+filter+")")
	}

	var warnings []string
	for _, name := range m.failingChecks {
		warnings = append(warnings, styleError.Render("⚠ healthcheck: "+name))
	}
	if len(m.errs) > 0 {
		msgs := make([]string, len(m.errs))
		for i, e := range m.errs {
			msgs[i] = e.Error()
		}
		warnings = append(warnings, styleError.Render("⚠ "+strings.Join(msgs, "; ")))
	}

	out := crumb
	if len(warnings) > 0 {
		out += "  " + strings.Join(warnings, "  ")
	}
	return styleBreadcrumb.Render(out)
}

func (m AppModel) renderFooter() string {
	if m.mode == modeFilter {
		return styleFooter.Render("filter: ") + m.filterInput.View() + styleFooter.Render("  <Enter> apply  <ESC> cancel")
	}
	if m.mode == modeCommand {
		return styleFooter.Render(":") + m.cmdInput.View()
	}

	hint := func(key, desc string) string {
		return styleFooterKey.Render("<"+key+">") + styleFooter.Render(desc)
	}
	parts := []string{
		hint(":", "cmd"),
		hint("/", "filter"),
		hint("Enter", "detail"),
		hint("r", "refresh"),
		hint("?", "help"),
		hint("q", "quit"),
	}
	return styleFooter.Render("  " + strings.Join(parts, "  "))
}

func fetchAlerts(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		alerts, errs := alertmanager.FetchAll(context.Background(), cfg)
		return alertsFetchedMsg{alerts: alerts, errs: errs}
	}
}

func scheduleTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// partitionHealthchecks separates alerts matched by healthcheck filter sets from
// regular alerts. An alert is hidden from the main list if it matches ALL filters
// in any one healthcheck set. A check is "failing" when no alerts match its filters.
func partitionHealthchecks(alerts []alertmanager.Alert, checks map[string][]string) (regular []alertmanager.Alert, failing []string) {
	if len(checks) == 0 {
		return alerts, nil
	}

	matched := make(map[string]bool, len(checks))
	for name := range checks {
		matched[name] = false
	}

	for _, a := range alerts {
		isHealthcheck := false
		for name, filters := range checks {
			if alertMatchesAllFilters(a, filters) {
				matched[name] = true
				isHealthcheck = true
			}
		}
		if !isHealthcheck {
			regular = append(regular, a)
		}
	}

	for name, ok := range matched {
		if !ok {
			failing = append(failing, name)
		}
	}
	sort.Strings(failing)
	return regular, failing
}

// alertMatchesAllFilters returns true when the alert satisfies every filter (AND logic).
func alertMatchesAllFilters(a alertmanager.Alert, filters []string) bool {
	for _, f := range filters {
		if !matchesFilter(a, f) {
			return false
		}
	}
	return true
}

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		days := int(d.Hours()) / 24
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours()) / (24 * 7)
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	default:
		months := int(d.Hours()) / (24 * 30)
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	}
}
