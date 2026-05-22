package config

import (
	"fmt"
	"regexp"
	"strconv"
)

// Theme holds the color values used by the TUI. Each field is either empty
// (inherit the default) or a valid lipgloss color: ANSI 256 (0–255) or hex (#RGB / #RRGGBB).
type Theme struct {
	Critical   string `yaml:"critical"`
	Warning    string `yaml:"warning"`
	Info       string `yaml:"info"`
	Good       string `yaml:"good"`
	Muted      string `yaml:"muted"`
	Accent     string `yaml:"accent"`
	HeaderBg   string `yaml:"header_bg"`
	HeaderFg   string `yaml:"header_fg"`
	InstanceBg string `yaml:"instance_bg"`
	SelectedBg string `yaml:"selected_bg"`
}

// DefaultTheme returns a Gruvbox Dark-based theme.
func DefaultTheme() Theme {
	return Theme{
		Critical:   "#fb4934",
		Warning:    "#fabd2f",
		Info:       "#83a598",
		Good:       "#b8bb26",
		Muted:      "#928374",
		Accent:     "#fe8019",
		HeaderBg:   "#3c3836",
		HeaderFg:   "#ebdbb2",
		InstanceBg: "#458588",
		SelectedBg: "#504945",
	}
}

var hexColorRE = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)

func validateColor(field, s string) error {
	if s == "" {
		return nil
	}
	if hexColorRE.MatchString(s) {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err == nil && n >= 0 && n <= 255 {
		return nil
	}
	return fmt.Errorf("theme field %q: %q is not a valid color (expected empty, #RGB, #RRGGBB, or 0–255)", field, s)
}

func (t Theme) validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{"critical", t.Critical},
		{"warning", t.Warning},
		{"info", t.Info},
		{"good", t.Good},
		{"muted", t.Muted},
		{"accent", t.Accent},
		{"header_bg", t.HeaderBg},
		{"header_fg", t.HeaderFg},
		{"instance_bg", t.InstanceBg},
		{"selected_bg", t.SelectedBg},
	}
	for _, f := range fields {
		if err := validateColor(f.name, f.value); err != nil {
			return err
		}
	}
	return nil
}

func fillThemeDefaults(t *Theme) {
	def := DefaultTheme()
	if t.Critical == "" {
		t.Critical = def.Critical
	}
	if t.Warning == "" {
		t.Warning = def.Warning
	}
	if t.Info == "" {
		t.Info = def.Info
	}
	if t.Good == "" {
		t.Good = def.Good
	}
	if t.Muted == "" {
		t.Muted = def.Muted
	}
	if t.Accent == "" {
		t.Accent = def.Accent
	}
	if t.HeaderBg == "" {
		t.HeaderBg = def.HeaderBg
	}
	if t.HeaderFg == "" {
		t.HeaderFg = def.HeaderFg
	}
	if t.InstanceBg == "" {
		t.InstanceBg = def.InstanceBg
	}
	if t.SelectedBg == "" {
		t.SelectedBg = def.SelectedBg
	}
}
