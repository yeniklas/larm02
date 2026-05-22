package alertmanager

import (
	"strings"
	"testing"
	"time"
)

func TestMatchersFromLabels(t *testing.T) {
	labels := map[string]string{
		"alertname": "HighCPU",
		"severity":  "critical",
		"instance":  "server-01",
	}
	matchers := MatchersFromLabels(labels)

	if len(matchers) != len(labels) {
		t.Fatalf("expected %d matchers, got %d", len(labels), len(matchers))
	}

	got := make(map[string]Matcher, len(matchers))
	for _, m := range matchers {
		got[m.Name] = m
	}

	for k, v := range labels {
		m, ok := got[k]
		if !ok {
			t.Errorf("missing matcher for label %q", k)
			continue
		}
		if m.Value != v {
			t.Errorf("matcher %q: want value %q, got %q", k, v, m.Value)
		}
		if m.IsRegex {
			t.Errorf("matcher %q: IsRegex should be false", k)
		}
		if !m.IsEqual {
			t.Errorf("matcher %q: IsEqual should be true", k)
		}
	}
}

func TestMatchersFromLabels_Empty(t *testing.T) {
	matchers := MatchersFromLabels(map[string]string{})
	if len(matchers) != 0 {
		t.Errorf("expected empty slice, got %d matchers", len(matchers))
	}
}

func TestRenderComment_SubstitutesNow(t *testing.T) {
	// RFC3339 truncates to seconds, so align the window to whole seconds.
	before := time.Now().UTC().Truncate(time.Second)
	result := RenderComment("ACK on %NOW%")
	after := time.Now().UTC().Add(time.Second).Truncate(time.Second)

	if strings.Contains(result, "%NOW%") {
		t.Error("expected %NOW% to be replaced, but it was not")
	}
	if !strings.HasPrefix(result, "ACK on ") {
		t.Errorf("unexpected prefix: %q", result)
	}

	ts := strings.TrimPrefix(result, "ACK on ")
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("timestamp not RFC3339: %q: %v", ts, err)
	}
	if parsed.Before(before) || parsed.After(after) {
		t.Errorf("timestamp %v not between %v and %v", parsed, before, after)
	}
}

func TestRenderComment_NoPlaceholder(t *testing.T) {
	result := RenderComment("static comment")
	if result != "static comment" {
		t.Errorf("expected unchanged, got %q", result)
	}
}
