package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write config: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
  - name: staging
    url: http://am.staging:9093
refresh_interval: 1m
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Alertmanagers) != 2 {
		t.Errorf("expected 2 alertmanagers, got %d", len(cfg.Alertmanagers))
	}
	if cfg.Alertmanagers[0].Name != "prod" {
		t.Errorf("first name: want %q, got %q", "prod", cfg.Alertmanagers[0].Name)
	}
	if cfg.GetRefreshInterval() != time.Minute {
		t.Errorf("refresh interval: want 1m, got %v", cfg.GetRefreshInterval())
	}
}

func TestLoad_DefaultRefreshInterval(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GetRefreshInterval() != 30*time.Second {
		t.Errorf("expected default 30s, got %v", cfg.GetRefreshInterval())
	}
}

func TestLoad_AcknowledgementDefaults(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Acknowledgement.GetDuration() != 15*time.Minute {
		t.Errorf("ack duration: want 15m, got %v", cfg.Acknowledgement.GetDuration())
	}
	if cfg.Acknowledgement.GetAuthor() != "larm02" {
		t.Errorf("ack author: want %q, got %q", "larm02", cfg.Acknowledgement.GetAuthor())
	}
	if cfg.Acknowledgement.GetComment() == "" {
		t.Error("ack comment default should not be empty")
	}
}

func TestLoad_AcknowledgementOverride(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
acknowledgement:
  duration: 30m
  author: oncall
  comment: "custom comment"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Acknowledgement.GetDuration() != 30*time.Minute {
		t.Errorf("ack duration: want 30m, got %v", cfg.Acknowledgement.GetDuration())
	}
	if cfg.Acknowledgement.GetAuthor() != "oncall" {
		t.Errorf("ack author: want %q, got %q", "oncall", cfg.Acknowledgement.GetAuthor())
	}
	if cfg.Acknowledgement.GetComment() != "custom comment" {
		t.Errorf("ack comment: want %q, got %q", "custom comment", cfg.Acknowledgement.GetComment())
	}
}

func TestLoad_MissingURL(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for missing URL, got nil")
	}
}

func TestLoad_NoAlertmanagers(t *testing.T) {
	path := writeConfig(t, `refresh_interval: 30s`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for empty alertmanagers list, got nil")
	}
}

func TestLoad_CustomColumns(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
columns:
  - label: team
    header: TEAM
    width: 14
  - label: env
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cfg.Columns))
	}

	col := cfg.Columns[0]
	if col.Label != "team" {
		t.Errorf("label: want %q, got %q", "team", col.Label)
	}
	if col.GetHeader() != "TEAM" {
		t.Errorf("header: want %q, got %q", "TEAM", col.GetHeader())
	}
	if col.GetWidth() != 14 {
		t.Errorf("width: want 14, got %d", col.GetWidth())
	}

	col2 := cfg.Columns[1]
	if col2.GetHeader() != "ENV" {
		t.Errorf("default header: want %q, got %q", "ENV", col2.GetHeader())
	}
	if col2.GetWidth() != 12 {
		t.Errorf("default width: want 12, got %d", col2.GetWidth())
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestAlertmanagerURL(t *testing.T) {
	cfg := &Config{
		Alertmanagers: []AlertmanagerConfig{
			{Name: "prod", URL: "http://prod:9093"},
			{Name: "staging", URL: "http://staging:9093"},
		},
	}

	if got := cfg.AlertmanagerURL("prod"); got != "http://prod:9093" {
		t.Errorf("prod: want %q, got %q", "http://prod:9093", got)
	}
	if got := cfg.AlertmanagerURL("staging"); got != "http://staging:9093" {
		t.Errorf("staging: want %q, got %q", "http://staging:9093", got)
	}
	if got := cfg.AlertmanagerURL("unknown"); got != "" {
		t.Errorf("unknown: expected empty string, got %q", got)
	}
}
