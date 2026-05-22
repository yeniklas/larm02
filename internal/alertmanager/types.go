package alertmanager

import (
	"strings"
	"time"
)

type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	EndsAt      time.Time         `json:"endsAt"`
	GeneratorURL string           `json:"generatorURL"`
	Fingerprint  string           `json:"fingerprint"`
	Receivers   []Receiver        `json:"receivers"`
	Status      AlertStatus       `json:"status"`

	// Instance is the Alertmanager instance name this alert came from (not from API).
	Instance string `json:"-"`
}

type AlertStatus struct {
	State       string   `json:"state"` // active | suppressed | unprocessed
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

type Receiver struct {
	Name string `json:"name"`
}

type AlertGroup struct {
	Labels   map[string]string `json:"labels"`
	Receiver Receiver          `json:"receiver"`
	Alerts   []Alert           `json:"alerts"`
}

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

type PostableSilence struct {
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

// MatchersFromLabels builds an exact-match Matcher slice from an alert's label set.
func MatchersFromLabels(labels map[string]string) []Matcher {
	matchers := make([]Matcher, 0, len(labels))
	for k, v := range labels {
		matchers = append(matchers, Matcher{
			Name:    k,
			Value:   v,
			IsRegex: false,
			IsEqual: true,
		})
	}
	return matchers
}

// RenderComment replaces %NOW% with the current UTC time.
func RenderComment(tmpl string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return strings.ReplaceAll(tmpl, "%NOW%", now)
}
