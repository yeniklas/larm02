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

func serveAlerts(alerts []Alert) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}
}

func cfgWithURLs(urls ...string) *config.Config {
	ams := make([]config.AlertmanagerConfig, len(urls))
	for i, u := range urls {
		ams[i] = config.AlertmanagerConfig{Name: "inst" + string(rune('0'+i+1)), URL: u}
	}
	return &config.Config{Alertmanagers: ams}
}

// TestFetchAll_HappyPath: single instance returns alerts, instance name is tagged.
func TestFetchAll_HappyPath(t *testing.T) {
	alerts := []Alert{
		makeAlert(map[string]string{"alertname": "HighCPU", "severity": "critical"}, "fp1"),
		makeAlert(map[string]string{"alertname": "DiskFull", "severity": "warning"}, "fp2"),
	}
	srv := httptest.NewServer(serveAlerts(alerts))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Alertmanagers: []config.AlertmanagerConfig{{Name: "prod", URL: srv.URL}},
	}
	got, errs := FetchAll(context.Background(), cfg)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(got))
	}
	for _, a := range got {
		if a.Instance != "prod" {
			t.Errorf("expected instance %q, got %q", "prod", a.Instance)
		}
	}
}

// TestFetchAll_MultiInstance: two instances, results from both are returned.
func TestFetchAll_MultiInstance(t *testing.T) {
	srv1 := httptest.NewServer(serveAlerts([]Alert{
		makeAlert(map[string]string{"alertname": "AlertA"}, "fp-a"),
	}))
	t.Cleanup(srv1.Close)

	srv2 := httptest.NewServer(serveAlerts([]Alert{
		makeAlert(map[string]string{"alertname": "AlertB"}, "fp-b"),
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
		t.Fatalf("expected 2 alerts (one per instance), got %d", len(got))
	}
}

// TestFetchAll_Deduplication: same instance name + same fingerprint → only one alert kept.
func TestFetchAll_Deduplication(t *testing.T) {
	// Two config entries sharing the same name produce the same instance tag,
	// so an identical fingerprint from both is considered a duplicate.
	alert := makeAlert(map[string]string{"alertname": "Watchdog"}, "fp-dup")
	srv1 := httptest.NewServer(serveAlerts([]Alert{alert}))
	t.Cleanup(srv1.Close)
	srv2 := httptest.NewServer(serveAlerts([]Alert{alert}))
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
		t.Errorf("expected 1 alert after deduplication, got %d", len(got))
	}
}

// TestFetchAll_OneInstanceFails: one 500, one healthy — healthy alerts returned, error collected.
func TestFetchAll_OneInstanceFails(t *testing.T) {
	goodAlerts := []Alert{makeAlert(map[string]string{"alertname": "OK"}, "fp-ok")}
	goodSrv := httptest.NewServer(serveAlerts(goodAlerts))
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
		t.Errorf("expected 1 alert from healthy instance, got %d", len(got))
	}
}

// TestFetchAll_ContextCancelled: cancelled context causes fetch to fail.
func TestFetchAll_ContextCancelled(t *testing.T) {
	// Server that blocks until the test ends.
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
		t.Errorf("expected no alerts, got %d", len(got))
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

	// Verify matchers cover all labels.
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
