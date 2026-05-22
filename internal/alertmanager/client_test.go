package alertmanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yeniklas/larm02/internal/config"
)

func makeAlert(labels map[string]string, fingerprint string) Alert {
	return Alert{
		Labels:      labels,
		Annotations: map[string]string{},
		Fingerprint: fingerprint,
		StartsAt:    time.Now().Add(-5 * time.Minute),
		Status:      AlertStatus{State: "active"},
	}
}

func makeGroup(groupLabels map[string]string, alerts []Alert) AlertGroup {
	return AlertGroup{
		Labels:   groupLabels,
		Receiver: Receiver{Name: "default"},
		Alerts:   alerts,
	}
}

func serveGroups(groups []AlertGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts/groups" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}
}

func countAlerts(groups []AlertGroup) int {
	n := 0
	for _, g := range groups {
		n += len(g.Alerts)
	}
	return n
}

func cfgWithURLs(urls ...string) *config.Config {
	ams := make([]config.AlertmanagerConfig, len(urls))
	for i, u := range urls {
		ams[i] = config.AlertmanagerConfig{Name: "inst" + string(rune('0'+i+1)), URL: u}
	}
	return &config.Config{Alertmanagers: ams}
}

// TestFetchAll_HappyPath: single instance returns groups, instance name is tagged on each alert.
func TestFetchAll_HappyPath(t *testing.T) {
	groups := []AlertGroup{
		makeGroup(map[string]string{"alertname": "HighCPU"}, []Alert{
			makeAlert(map[string]string{"alertname": "HighCPU", "severity": "critical"}, "fp1"),
		}),
		makeGroup(map[string]string{"alertname": "DiskFull"}, []Alert{
			makeAlert(map[string]string{"alertname": "DiskFull", "severity": "warning"}, "fp2"),
		}),
	}
	srv := httptest.NewServer(serveGroups(groups))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{{Name: "prod", URL: srv.URL}},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	for _, g := range got {
		for _, a := range g.Alerts {
			if a.Instance != "prod" {
				t.Errorf("expected instance %q, got %q", "prod", a.Instance)
			}
		}
	}
}

// TestFetchAll_MultiInstance: two instances with different groups — all groups returned.
func TestFetchAll_MultiInstance(t *testing.T) {
	srv1 := httptest.NewServer(serveGroups([]AlertGroup{
		makeGroup(map[string]string{"alertname": "AlertA"}, []Alert{
			makeAlert(map[string]string{"alertname": "AlertA"}, "fp-a"),
		}),
	}))
	t.Cleanup(srv1.Close)

	srv2 := httptest.NewServer(serveGroups([]AlertGroup{
		makeGroup(map[string]string{"alertname": "AlertB"}, []Alert{
			makeAlert(map[string]string{"alertname": "AlertB"}, "fp-b"),
		}),
	}))
	t.Cleanup(srv2.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{
			{Name: "inst1", URL: srv1.URL},
			{Name: "inst2", URL: srv2.URL},
		},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups (different labels), got %d", len(got))
	}
	if countAlerts(got) != 2 {
		t.Fatalf("expected 2 total alerts, got %d", countAlerts(got))
	}
}

// TestFetchAll_Deduplication: same instance name + same group labels + same fingerprint → one alert kept.
func TestFetchAll_Deduplication(t *testing.T) {
	alert := makeAlert(map[string]string{"alertname": "Watchdog"}, "fp-dup")
	group := makeGroup(map[string]string{"alertname": "Watchdog"}, []Alert{alert})

	srv1 := httptest.NewServer(serveGroups([]AlertGroup{group}))
	t.Cleanup(srv1.Close)
	srv2 := httptest.NewServer(serveGroups([]AlertGroup{group}))
	t.Cleanup(srv2.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{
			{Name: "shared", URL: srv1.URL},
			{Name: "shared", URL: srv2.URL},
		},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 merged group, got %d", len(got))
	}
	if len(got) > 0 && len(got[0].Alerts) != 1 {
		t.Errorf("expected 1 alert after deduplication, got %d", len(got[0].Alerts))
	}
}

// TestFetchAll_GroupMerge: two instances reporting the same group labels get merged into one group.
func TestFetchAll_GroupMerge(t *testing.T) {
	a1 := makeAlert(map[string]string{"alertname": "HighCPU"}, "fp-1")
	a2 := makeAlert(map[string]string{"alertname": "HighCPU"}, "fp-2")
	groupLabels := map[string]string{"alertname": "HighCPU"}

	srv1 := httptest.NewServer(serveGroups([]AlertGroup{
		makeGroup(groupLabels, []Alert{a1}),
	}))
	t.Cleanup(srv1.Close)
	srv2 := httptest.NewServer(serveGroups([]AlertGroup{
		makeGroup(groupLabels, []Alert{a2}),
	}))
	t.Cleanup(srv2.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{
			{Name: "inst1", URL: srv1.URL},
			{Name: "inst2", URL: srv2.URL},
		},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 1 {
		t.Fatalf("expected groups with same labels to be merged into 1, got %d", len(got))
	}
	if len(got[0].Alerts) != 2 {
		t.Errorf("expected 2 alerts in merged group, got %d", len(got[0].Alerts))
	}
}

// TestFetchAll_OneInstanceFails: one 500, one healthy — healthy groups returned, error collected.
func TestFetchAll_OneInstanceFails(t *testing.T) {
	goodGroups := []AlertGroup{
		makeGroup(map[string]string{"alertname": "OK"}, []Alert{
			makeAlert(map[string]string{"alertname": "OK"}, "fp-ok"),
		}),
	}
	goodSrv := httptest.NewServer(serveGroups(goodGroups))
	t.Cleanup(goodSrv.Close)

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(badSrv.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{
			{Name: "good", URL: goodSrv.URL},
			{Name: "bad", URL: badSrv.URL},
		},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
	if len(got) != 1 {
		t.Errorf("expected 1 group from healthy instance, got %d", len(got))
	}
}

// TestFetchAll_ContextCancelled: cancelled context causes fetch to fail.
func TestFetchAll_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{{Name: "slow", URL: srv.URL}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	got, errs := FetchAll(ctx, cfg)

	if len(errs) == 0 {
		t.Error("expected an error from cancelled context, got none")
	}
	if len(got) != 0 {
		t.Errorf("expected no groups, got %d", len(got))
	}
}

// TestPostSilence_HappyPath: verifies correct JSON body is sent to the server.
func TestPostSilence_HappyPath(t *testing.T) {
	var received PostableSilence

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/silences" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"silenceID":"abc-123"}`))
	}))
	t.Cleanup(srv.Close)

	alert := makeAlert(map[string]string{"alertname": "HighCPU", "severity": "critical"}, "fp1")
	ackCfg := config.AcknowledgementConfig{
		Duration: "15m",
		Author:   "tester",
		Comment:  "ACK on %NOW%",
	}

	err := PostSilence(context.Background(), srv.URL, alert, ackCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.CreatedBy != "tester" {
		t.Errorf("CreatedBy: want %q, got %q", "tester", received.CreatedBy)
	}
	if received.Comment == "" || received.Comment == "ACK on %NOW%" {
		t.Errorf("Comment placeholder not replaced: %q", received.Comment)
	}
	if received.EndsAt.Sub(received.StartsAt) < 14*time.Minute {
		t.Errorf("silence duration too short: %v", received.EndsAt.Sub(received.StartsAt))
	}

	gotMatchers := make(map[string]string, len(received.Matchers))
	for _, m := range received.Matchers {
		gotMatchers[m.Name] = m.Value
	}
	for k, v := range alert.Labels {
		if gotMatchers[k] != v {
			t.Errorf("matcher %q: want %q, got %q", k, v, gotMatchers[k])
		}
	}
}

// TestPostSilence_ServerError: non-200 response is returned as an error.
func TestPostSilence_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	alert := makeAlert(map[string]string{"alertname": "X"}, "fp")
	err := PostSilence(context.Background(), srv.URL, alert, config.AcknowledgementConfig{})
	if err == nil {
		t.Error("expected an error for non-200 response, got nil")
	}
}
