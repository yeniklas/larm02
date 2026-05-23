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

var logoText = `    __                         ____ ___ ` + "\n" +
	`   / /   ____ __________ ___  / __ \__ \` + "\n" +
	`  / /   / __ ` + "`" + `/ ___/ __ ` + "`" + `__ \/ / / /_/ /` + "\n" +
	` / /___/ /_/ / /  / / / / / / /_/ / __/ ` + "\n" +
	`/_____/\__,_/_/  /_/ /_/ /_/\____/____/`

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeCommand
	modeDetail
	modeGroupDetail
	modeHelp
	modeInstances
)

type displayItemKind int

const (
	displayItemAlert displayItemKind = iota
	displayItemGroup
	displayItemSection
)

type displayItem struct {
	kind         displayItemKind
	alert        alertmanager.Alert
	group        alertmanager.AlertGroup
	alerts       []alertmanager.Alert
	sectionLabel string
	sectionValue string
	groupCount   int
	collapsed    bool
}

type alertsFetchedMsg struct {
	groups []alertmanager.AlertGroup
	errs   []error
}

type silencePostedMsg struct{ err error }

type tickMsg time.Time

// AppModel is the root bubbletea model.
type AppModel struct {
	cfg             *config.Config
	groups          []alertmanager.AlertGroup // raw groups from API
	alerts          []alertmanager.Alert      // flattened regular alerts (post-healthcheck partition)
	items           []displayItem             // filtered display list navigated by cursor
	failingChecks   []string                  // healthcheck names with no matching alerts
	cursor          int
	mode            mode
	filterInput     textinput.Model
	cmdInput        textinput.Model
	spinner         spinner.Model
	loading         bool
	lastRefresh     time.Time
	errs            []error
	statusMsg       string // transient feedback shown in footer, cleared on next fetch
	width           int
	height          int
	hiddenInstances   map[string]bool
	instanceCursor    int
	groupDetailCursor int // cursor within group detail alert list
	groupLabelIdx     int             // -1 = off, 0..n-1 = active label index
	collapsedSections map[string]bool // keyed by sectionValue; true = collapsed
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

	hidden := make(map[string]bool)
	for _, am := range cfg.Alertmanagers {
		if am.Hidden {
			hidden[am.Name] = true
		}
	}

	return AppModel{
		cfg:               cfg,
		filterInput:       fi,
		cmdInput:          ci,
		spinner:           sp,
		loading:           true,
		hiddenInstances:   hidden,
		groupLabelIdx:     -1,
		collapsedSections: make(map[string]bool),
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
		m.groups = msg.groups
		m.alerts, m.failingChecks = partitionHealthchecks(flattenGroups(msg.groups), m.cfg.Healthchecks)
		m.errs = msg.errs
		m.statusMsg = ""
		m.applyFilter()
		if m.cursor >= len(m.items) && m.cursor > 0 {
			m.cursor = len(m.items) - 1
		}
		return m, nil

	case silencePostedMsg:
		if msg.err != nil {
			m.statusMsg = styleError.Render("Ack failed: " + msg.err.Error())
		} else {
			m.statusMsg = lipgloss.NewStyle().Foreground(colorGood).Render("Acknowledged.")
			m.loading = true
			return m, fetchAlerts(m.cfg)
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
	case modeGroupDetail:
		return m.handleGroupDetailKey(msg)
	case modeHelp:
		if msg.Type == tea.KeyEsc || msg.String() == "?" || msg.String() == "q" {
			m.mode = modeNormal
		}
		return m, nil
	case modeInstances:
		return m.handleInstancesKey(msg)
	}
	return m.handleNormalKey(msg)
}

func (m AppModel) handleInstancesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "i", "q":
		m.mode = modeNormal
	case "j", "down":
		if m.instanceCursor < len(m.cfg.Alertmanagers)-1 {
			m.instanceCursor++
		}
	case "k", "up":
		if m.instanceCursor > 0 {
			m.instanceCursor--
		}
	case " ":
		name := m.cfg.Alertmanagers[m.instanceCursor].Name
		m.hiddenInstances[name] = !m.hiddenInstances[name]
		m.applyFilter()
	}
	return m, nil
}

func (m AppModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.items) > 0 {
			switch m.items[m.cursor].kind {
			case displayItemSection:
				val := m.items[m.cursor].sectionValue
				m.collapsedSections[val] = !m.collapsedSections[val]
				m.applyFilter()
				if m.cursor >= len(m.items) {
					m.cursor = max(0, len(m.items)-1)
				}
			case displayItemGroup:
				m.groupDetailCursor = 0
				m.mode = modeGroupDetail
			}
		}
	case "g":
		if len(m.cfg.GroupLabels) > 0 {
			if m.groupLabelIdx >= len(m.cfg.GroupLabels)-1 {
				m.groupLabelIdx = -1
			} else {
				m.groupLabelIdx++
			}
			m.collapsedSections = make(map[string]bool)
			m.applyFilter()
			m.cursor = 0
		}
	case " ":
		if len(m.items) > 0 && m.items[m.cursor].kind == displayItemSection {
			val := m.items[m.cursor].sectionValue
			m.collapsedSections[val] = !m.collapsedSections[val]
			m.applyFilter()
			if m.cursor >= len(m.items) {
				m.cursor = max(0, len(m.items)-1)
			}
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
	case "i":
		m.mode = modeInstances
		m.instanceCursor = 0
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

func (m AppModel) handleGroupDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.items) == 0 {
		m.mode = modeNormal
		return m, nil
	}
	alerts := m.items[m.cursor].alerts
	switch msg.String() {
	case "esc", "q":
		m.groupDetailCursor = 0
		m.mode = modeNormal
	case "j", "down":
		if m.groupDetailCursor < len(alerts)-1 {
			m.groupDetailCursor++
		}
	case "k", "up":
		if m.groupDetailCursor > 0 {
			m.groupDetailCursor--
		}
	case "enter":
		if len(alerts) > 0 {
			m.mode = modeDetail
		}
	case "a":
		if len(alerts) > 0 {
			return m, acknowledgeAlert(alerts[m.groupDetailCursor], m.cfg)
		}
	}
	return m, nil
}

func (m AppModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeGroupDetail
	case "a":
		if m.cursor < len(m.items) && len(m.items[m.cursor].alerts) > 0 {
			return m, acknowledgeAlert(m.items[m.cursor].alerts[m.groupDetailCursor], m.cfg)
		}
	}
	return m, nil
}

// applyFilter rebuilds m.items from m.groups, applying the current filter query,
// hidden instances, and healthcheck exclusions.
func (m *AppModel) applyFilter() {
	regularSet := make(map[string]struct{}, len(m.alerts))
	for _, a := range m.alerts {
		regularSet[a.Instance+"/"+a.Fingerprint] = struct{}{}
	}
	query := strings.TrimSpace(m.filterInput.Value())

	if m.groupLabelIdx < 0 {
		var items []displayItem
		for _, g := range m.groups {
			var matching []alertmanager.Alert
			for _, a := range g.Alerts {
				if _, ok := regularSet[a.Instance+"/"+a.Fingerprint]; !ok {
					continue
				}
				if m.hiddenInstances[a.Instance] {
					continue
				}
				if query != "" && !matchesFilter(a, query) {
					continue
				}
				matching = append(matching, a)
			}
			if len(matching) > 0 {
				items = append(items, displayItem{kind: displayItemGroup, group: g, alerts: matching})
			}
		}
		m.items = items
		return
	}

	// Section-grouping mode.
	activeLabel := m.cfg.GroupLabels[m.groupLabelIdx]
	type groupEntry struct {
		group  alertmanager.AlertGroup
		alerts []alertmanager.Alert
	}
	sections := make(map[string][]groupEntry)

	for _, g := range m.groups {
		byVal := make(map[string][]alertmanager.Alert)
		for _, a := range g.Alerts {
			if _, ok := regularSet[a.Instance+"/"+a.Fingerprint]; !ok {
				continue
			}
			if m.hiddenInstances[a.Instance] {
				continue
			}
			if query != "" && !matchesFilter(a, query) {
				continue
			}
			val := a.Labels[activeLabel]
			if val == "" {
				val = "(none)"
			}
			byVal[val] = append(byVal[val], a)
		}
		for val, alerts := range byVal {
			sections[val] = append(sections[val], groupEntry{group: g, alerts: alerts})
		}
	}

	sectionValues := make([]string, 0, len(sections))
	for v := range sections {
		sectionValues = append(sectionValues, v)
	}
	sort.Slice(sectionValues, func(i, j int) bool {
		if sectionValues[i] == "(none)" {
			return false
		}
		if sectionValues[j] == "(none)" {
			return true
		}
		return sectionValues[i] < sectionValues[j]
	})

	var items []displayItem
	for _, val := range sectionValues {
		entries := sections[val]
		collapsed := m.collapsedSections[val]
		items = append(items, displayItem{
			kind:         displayItemSection,
			sectionLabel: activeLabel,
			sectionValue: val,
			groupCount:   len(entries),
			collapsed:    collapsed,
		})
		if !collapsed {
			for _, e := range entries {
				items = append(items, displayItem{kind: displayItemGroup, group: e.group, alerts: e.alerts})
			}
		}
	}
	m.items = items
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
	if m.mode == modeInstances {
		return m.renderInstancesOverlay()
	}
	if m.mode == modeDetail && len(m.items) > 0 {
		return m.renderDetailView()
	}
	if m.mode == modeGroupDetail && len(m.items) > 0 {
		return m.renderGroupDetailView()
	}
	return m.renderMain()
}

func (m AppModel) renderInstancesOverlay() string {
	header := m.renderHeader()
	counts := countActiveAlertsByInstance(m.alerts)
	overlay := renderInstances(m.cfg.Alertmanagers, m.hiddenInstances, counts, m.instanceCursor, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, overlay)
}

func (m AppModel) renderMain() string {
	header := m.renderHeader()
	breadcrumb := m.renderBreadcrumb()
	footer := m.renderFooter()

	fixed := lipgloss.Height(header) + lipgloss.Height(breadcrumb) + lipgloss.Height(footer)
	parts := []string{header}

	if !m.cfg.DisableLogo {
		logo := m.renderLogo()
		fixed += lipgloss.Height(logo)
		parts = append(parts, logo)
	}

	parts = append(parts, breadcrumb)

	tableH := m.height - fixed
	if tableH < 0 {
		tableH = 0
	}
	table := renderAlertsTable(m.items, m.cursor, m.width, tableH, m.loading, m.spinner, m.cfg.Columns)
	parts = append(parts, table, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m AppModel) renderLogo() string {
	return lipgloss.NewStyle().Foreground(colorAccent).PaddingLeft(1).Render(logoText)
}

func (m AppModel) renderDetailView() string {
	header := m.renderHeader()
	alert := m.items[m.cursor].alerts[m.groupDetailCursor]
	detail := renderDetail(alert, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, detail)
}

func (m AppModel) renderGroupDetailView() string {
	header := m.renderHeader()
	group := m.items[m.cursor]

	keys := sortedKeys(group.group.Labels)
	labelParts := make([]string, 0, len(keys))
	for _, k := range keys {
		labelParts = append(labelParts, k+"="+group.group.Labels[k])
	}
	title := styleSectionHeader.Render("Group: " + strings.Join(labelParts, "  "))

	groupItems := make([]displayItem, len(group.alerts))
	for i, a := range group.alerts {
		groupItems[i] = displayItem{kind: displayItemAlert, alert: a}
	}

	hint := styleFooter.Render("  <Enter> detail  <a> ack  <j/k> navigate  <ESC> back")
	fixed := lipgloss.Height(header) + lipgloss.Height(title) + lipgloss.Height(hint)
	tableH := m.height - fixed
	if tableH < 0 {
		tableH = 0
	}
	table := renderAlertsTable(groupItems, m.groupDetailCursor, m.width, tableH, false, m.spinner, m.cfg.Columns)

	return lipgloss.JoinVertical(lipgloss.Left, header, title, table, hint)
}

func (m AppModel) renderHelp() string {
	header := m.renderHeader()
	help := renderHelp(m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, help)
}

func (m AppModel) renderHeader() string {
	counts := countActiveAlertsByInstance(m.alerts)
	left := styleHeader.Render("larm02")
	for _, am := range m.cfg.Alertmanagers {
		s := styleInstance
		if m.hiddenInstances[am.Name] {
			s = styleInstanceHidden
		}
		left += s.Render(fmt.Sprintf("%s (%d)", am.Name, counts[am.Name]))
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
	if m.groupLabelIdx >= 0 && m.groupLabelIdx < len(m.cfg.GroupLabels) {
		crumb += "  " + styleFilter.Render("(grouped by: "+m.cfg.GroupLabels[m.groupLabelIdx]+")")
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

	if m.statusMsg != "" {
		return styleFooter.Render("  ") + m.statusMsg
	}

	hint := func(key, desc string) string {
		return styleFooterKey.Render("<"+key+">") + styleFooter.Render(desc)
	}
	parts := []string{
		hint(":", "cmd"),
		hint("/", "filter"),
		hint("Enter", "detail"),
		hint("a", "ack"),
		hint("r", "refresh"),
		hint("g", "group"),
		hint("i", "instances"),
		hint("?", "help"),
		hint("q", "quit"),
	}
	return styleFooter.Render("  " + strings.Join(parts, "  "))
}

func acknowledgeAlert(alert alertmanager.Alert, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		baseURL := cfg.AlertmanagerURL(alert.Instance)
		if baseURL == "" {
			return silencePostedMsg{fmt.Errorf("unknown instance %q", alert.Instance)}
		}
		err := alertmanager.PostSilence(context.Background(), baseURL, alert, cfg.Acknowledgement)
		return silencePostedMsg{err}
	}
}

func fetchAlerts(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		groups, errs := alertmanager.FetchAll(context.Background(), cfg)
		return alertsFetchedMsg{groups: groups, errs: errs}
	}
}

func flattenGroups(groups []alertmanager.AlertGroup) []alertmanager.Alert {
	var alerts []alertmanager.Alert
	for _, g := range groups {
		alerts = append(alerts, g.Alerts...)
	}
	return alerts
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

func countActiveAlertsByInstance(alerts []alertmanager.Alert) map[string]int {
	counts := make(map[string]int)
	for _, a := range alerts {
		if a.Status.State == "active" {
			counts[a.Instance]++
		}
	}
	return counts
}
