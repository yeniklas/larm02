package ui

import (
	"testing"
	"time"

	"github.com/yeniklas/larm02/internal/alertmanager"
	"github.com/yeniklas/larm02/internal/config"
)

func alert(labels map[string]string) alertmanager.Alert {
	return alertmanager.Alert{
		Labels:      labels,
		Annotations: map[string]string{},
		Status:      alertmanager.AlertStatus{State: "active"},
		Instance:    "prod",
	}
}

// --- matchesFilter ---

func TestMatchesFilter_ExactLabel(t *testing.T) {
	a := alert(map[string]string{"severity": "critical", "alertname": "HighCPU"})
	if !matchesFilter(a, "severity=critical") {
		t.Error("expected match for exact label")
	}
	if matchesFilter(a, "severity=warning") {
		t.Error("expected no match for wrong value")
	}
}

func TestMatchesFilter_CaseInsensitive(t *testing.T) {
	a := alert(map[string]string{"severity": "Critical"})
	if !matchesFilter(a, "severity=critical") {
		t.Error("expected case-insensitive match")
	}
}

func TestMatchesFilter_SubstringValue(t *testing.T) {
	a := alert(map[string]string{"alertname": "HighCPUUsage"})
	if !matchesFilter(a, "alertname=~cpu") {
		t.Error("expected substring match with =~")
	}
	if matchesFilter(a, "alertname=~disk") {
		t.Error("expected no match for unrelated substring")
	}
}

func TestMatchesFilter_PlainSubstring(t *testing.T) {
	a := alert(map[string]string{"alertname": "HighCPU"})
	a.Instance = "production"
	if !matchesFilter(a, "cpu") {
		t.Error("expected plain substring match on alertname")
	}
	if !matchesFilter(a, "prod") {
		t.Error("expected plain substring match on instance")
	}
	if matchesFilter(a, "disk") {
		t.Error("expected no match for unrelated term")
	}
}

func TestMatchesFilter_MissingLabel(t *testing.T) {
	a := alert(map[string]string{"alertname": "X"})
	if matchesFilter(a, "severity=critical") {
		t.Error("expected no match when label is absent")
	}
}

func TestMatchesFilter_Annotation(t *testing.T) {
	a := alertmanager.Alert{
		Labels:      map[string]string{"alertname": "X"},
		Annotations: map[string]string{"summary": "disk is full"},
		Status:      alertmanager.AlertStatus{State: "active"},
	}
	if !matchesFilter(a, "summary=disk is full") {
		t.Error("expected match on annotation value")
	}
}

// --- alertMatchesAllFilters ---

func TestAlertMatchesAllFilters_AllMatch(t *testing.T) {
	a := alert(map[string]string{"alertname": "X", "severity": "critical", "env": "prod"})
	filters := []string{"severity=critical", "env=prod"}
	if !alertMatchesAllFilters(a, filters) {
		t.Error("expected all filters to match")
	}
}

func TestAlertMatchesAllFilters_OneMiss(t *testing.T) {
	a := alert(map[string]string{"alertname": "X", "severity": "warning"})
	filters := []string{"severity=critical", "alertname=X"}
	if alertMatchesAllFilters(a, filters) {
		t.Error("expected false when one filter misses")
	}
}

func TestAlertMatchesAllFilters_EmptyFilters(t *testing.T) {
	a := alert(map[string]string{"alertname": "X"})
	if !alertMatchesAllFilters(a, nil) {
		t.Error("empty filter list should match everything")
	}
}

// --- partitionHealthchecks ---

func TestPartitionHealthchecks_NoChecks(t *testing.T) {
	alerts := []alertmanager.Alert{
		alert(map[string]string{"alertname": "A"}),
		alert(map[string]string{"alertname": "B"}),
	}
	regular, failing := partitionHealthchecks(alerts, nil)
	if len(regular) != 2 {
		t.Errorf("expected 2 regular alerts, got %d", len(regular))
	}
	if len(failing) != 0 {
		t.Errorf("expected no failing checks, got %v", failing)
	}
}

func TestPartitionHealthchecks_PassingCheck(t *testing.T) {
	watchdog := alert(map[string]string{"alertname": "Watchdog", "severity": "none"})
	other := alert(map[string]string{"alertname": "RealAlert", "severity": "critical"})
	checks := map[string][]string{
		"watchdog": {"alertname=Watchdog"},
	}
	regular, failing := partitionHealthchecks([]alertmanager.Alert{watchdog, other}, checks)

	if len(regular) != 1 || regular[0].Labels["alertname"] != "RealAlert" {
		t.Errorf("watchdog alert should be hidden from regular list, got %v", regular)
	}
	if len(failing) != 0 {
		t.Errorf("check should be passing, got failing: %v", failing)
	}
}

func TestPartitionHealthchecks_FailingCheck(t *testing.T) {
	other := alert(map[string]string{"alertname": "RealAlert"})
	checks := map[string][]string{
		"watchdog": {"alertname=Watchdog"},
	}
	regular, failing := partitionHealthchecks([]alertmanager.Alert{other}, checks)

	if len(regular) != 1 {
		t.Errorf("expected 1 regular alert, got %d", len(regular))
	}
	if len(failing) != 1 || failing[0] != "watchdog" {
		t.Errorf("expected [watchdog] in failing, got %v", failing)
	}
}

func TestPartitionHealthchecks_MultipleChecks(t *testing.T) {
	watchdog := alert(map[string]string{"alertname": "Watchdog"})
	// infra-watchdog has no matching alert → failing
	checks := map[string][]string{
		"watchdog":       {"alertname=Watchdog"},
		"infra-watchdog": {"alertname=InfraWatchdog"},
	}
	regular, failing := partitionHealthchecks([]alertmanager.Alert{watchdog}, checks)

	if len(regular) != 0 {
		t.Errorf("expected all alerts hidden by healthchecks, got %d regular", len(regular))
	}
	if len(failing) != 1 || failing[0] != "infra-watchdog" {
		t.Errorf("expected [infra-watchdog] failing, got %v", failing)
	}
}

func TestPartitionHealthchecks_FailingSorted(t *testing.T) {
	checks := map[string][]string{
		"zzz": {"alertname=Missing"},
		"aaa": {"alertname=Missing"},
		"mmm": {"alertname=Missing"},
	}
	_, failing := partitionHealthchecks(nil, checks)

	if len(failing) != 3 {
		t.Fatalf("expected 3 failing checks, got %d", len(failing))
	}
	for i := 1; i < len(failing); i++ {
		if failing[i] < failing[i-1] {
			t.Errorf("failing checks not sorted: %v", failing)
		}
	}
}

// --- instancesFailingHealthchecks ---

func TestInstancesFailingHealthchecks_NoChecks(t *testing.T) {
	ams := []config.AlertmanagerConfig{{Name: "prod"}}
	result := instancesFailingHealthchecks(nil, nil, ams)
	if result != nil {
		t.Errorf("expected nil when no checks configured, got %v", result)
	}
}

func TestInstancesFailingHealthchecks_AllHealthy(t *testing.T) {
	ams := []config.AlertmanagerConfig{{Name: "prod"}, {Name: "staging"}}
	alerts := []alertmanager.Alert{
		{Labels: map[string]string{"alertname": "Watchdog"}, Instance: "prod", Status: alertmanager.AlertStatus{State: "active"}, Annotations: map[string]string{}},
		{Labels: map[string]string{"alertname": "Watchdog"}, Instance: "staging", Status: alertmanager.AlertStatus{State: "active"}, Annotations: map[string]string{}},
	}
	checks := map[string][]string{"watchdog": {"alertname=Watchdog"}}
	result := instancesFailingHealthchecks(alerts, checks, ams)
	if len(result) != 0 {
		t.Errorf("expected all instances healthy, got unhealthy: %v", result)
	}
}

func TestInstancesFailingHealthchecks_OneUnhealthy(t *testing.T) {
	ams := []config.AlertmanagerConfig{{Name: "prod"}, {Name: "staging"}}
	alerts := []alertmanager.Alert{
		{Labels: map[string]string{"alertname": "Watchdog"}, Instance: "prod", Status: alertmanager.AlertStatus{State: "active"}, Annotations: map[string]string{}},
		// staging has no Watchdog
	}
	checks := map[string][]string{"watchdog": {"alertname=Watchdog"}}
	result := instancesFailingHealthchecks(alerts, checks, ams)
	if !result["staging"] {
		t.Error("expected staging to be unhealthy")
	}
	if result["prod"] {
		t.Error("expected prod to be healthy")
	}
}

// --- humanDuration ---

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{90 * time.Minute, "1h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
		{6 * 24 * time.Hour, "6 days"},
		{7 * 24 * time.Hour, "1 week"},
		{14 * 24 * time.Hour, "2 weeks"},
		{29 * 24 * time.Hour, "4 weeks"},
		{30 * 24 * time.Hour, "1 month"},
		{60 * 24 * time.Hour, "2 months"},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			got := humanDuration(tt.d)
			if got != tt.want {
				t.Errorf("humanDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
