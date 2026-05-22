package alertmanager

import "time"

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
