package config

import (
	"strings"
	"testing"
)

// --- validateColor ---

func TestValidateColor_empty(t *testing.T) {
	if err := validateColor("f", ""); err != nil {
		t.Errorf("empty should be valid, got %v", err)
	}
}

func TestValidateColor_ansi256(t *testing.T) {
	for _, s := range []string{"0", "1", "196", "255"} {
		if err := validateColor("f", s); err != nil {
			t.Errorf("%q should be valid ANSI 256, got %v", s, err)
		}
	}
}

func TestValidateColor_ansi256_outOfRange(t *testing.T) {
	for _, s := range []string{"256", "999", "-1"} {
		if err := validateColor("f", s); err == nil {
			t.Errorf("%q should be invalid (out of 0–255 range)", s)
		}
	}
}

func TestValidateColor_hexShort(t *testing.T) {
	if err := validateColor("f", "#ABC"); err != nil {
		t.Errorf("#ABC should be valid, got %v", err)
	}
}

func TestValidateColor_hexFull(t *testing.T) {
	if err := validateColor("f", "#fb4934"); err != nil {
		t.Errorf("#fb4934 should be valid, got %v", err)
	}
}

func TestValidateColor_hexUppercase(t *testing.T) {
	if err := validateColor("f", "#AABBCC"); err != nil {
		t.Errorf("#AABBCC should be valid, got %v", err)
	}
}

func TestValidateColor_hexBadLength(t *testing.T) {
	for _, s := range []string{"#AB", "#ABCD", "#ABCDE", "#ABCDEFG"} {
		if err := validateColor("f", s); err == nil {
			t.Errorf("%q should be invalid (wrong hex length)", s)
		}
	}
}

func TestValidateColor_hexNoHash(t *testing.T) {
	if err := validateColor("f", "AABBCC"); err == nil {
		t.Error("hex without # should be invalid")
	}
}

func TestValidateColor_namedColor(t *testing.T) {
	for _, s := range []string{"red", "blue", "primary", "auto"} {
		if err := validateColor("f", s); err == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

// --- DefaultTheme ---

func TestDefaultTheme_allFieldsNonEmpty(t *testing.T) {
	def := DefaultTheme()
	fields := map[string]string{
		"Critical":   def.Critical,
		"Warning":    def.Warning,
		"Info":       def.Info,
		"Good":       def.Good,
		"Muted":      def.Muted,
		"Accent":     def.Accent,
		"HeaderBg":   def.HeaderBg,
		"HeaderFg":   def.HeaderFg,
		"InstanceBg": def.InstanceBg,
		"SelectedBg": def.SelectedBg,
	}
	for name, val := range fields {
		if val == "" {
			t.Errorf("DefaultTheme().%s is empty", name)
		}
		if err := validateColor(name, val); err != nil {
			t.Errorf("DefaultTheme().%s = %q is not a valid color: %v", name, val, err)
		}
	}
}

// --- Theme via config.Load ---

func TestLoad_ThemeDefaults_WhenAbsent(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := DefaultTheme()
	if cfg.Theme != def {
		t.Errorf("expected default theme when none set\nwant: %+v\ngot:  %+v", def, cfg.Theme)
	}
}

func TestLoad_ThemePartial_FillsDefaults(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
theme:
  critical: "#ff0000"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme.Critical != "#ff0000" {
		t.Errorf("critical: want #ff0000, got %q", cfg.Theme.Critical)
	}
	def := DefaultTheme()
	if cfg.Theme.Muted != def.Muted {
		t.Errorf("unset Muted should be default %q, got %q", def.Muted, cfg.Theme.Muted)
	}
	if cfg.Theme.Accent != def.Accent {
		t.Errorf("unset Accent should be default %q, got %q", def.Accent, cfg.Theme.Accent)
	}
}

func TestLoad_ThemeInvalidColor(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
theme:
  critical: "notacolor"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid color, got nil")
	}
	if !strings.Contains(err.Error(), "critical") {
		t.Errorf("error should name the offending field, got: %v", err)
	}
}

func TestLoad_ThemeInvalidHex(t *testing.T) {
	path := writeConfig(t, `
alertmanagers:
  - name: prod
    url: http://am.prod:9093
theme:
  warning: "#GGGGGG"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hex, got nil")
	}
	if !strings.Contains(err.Error(), "warning") {
		t.Errorf("error should name the offending field, got: %v", err)
	}
}
