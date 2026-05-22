package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Alertmanagers   []AlertmanagerConfig  `yaml:"alertmanagers"`
	RefreshInterval duration              `yaml:"refresh_interval"`
	Healthchecks    map[string][]string   `yaml:"healthchecks"`
	Acknowledgement AcknowledgementConfig `yaml:"acknowledgement"`
	Columns         []ColumnConfig        `yaml:"columns"`
	Theme           Theme                 `yaml:"theme"`
	DisableLogo     bool                  `yaml:"disable_logo"`
}

// ColumnConfig defines an extra column in the alerts table backed by a label value.
type ColumnConfig struct {
	Label  string `yaml:"label"`
	Header string `yaml:"header"`
	Width  int    `yaml:"width"`
}

func (c ColumnConfig) GetHeader() string {
	if c.Header != "" {
		return c.Header
	}
	return strings.ToUpper(c.Label)
}

func (c ColumnConfig) GetWidth() int {
	if c.Width > 0 {
		return c.Width
	}
	return 12
}

type AcknowledgementConfig struct {
	Duration string `yaml:"duration"`
	Author   string `yaml:"author"`
	Comment  string `yaml:"comment"`
}

func (a AcknowledgementConfig) GetDuration() time.Duration {
	if d, err := time.ParseDuration(a.Duration); err == nil && d > 0 {
		return d
	}
	return 15 * time.Minute
}

func (a AcknowledgementConfig) GetAuthor() string {
	if a.Author != "" {
		return a.Author
	}
	return "larm02"
}

func (a AcknowledgementConfig) GetComment() string {
	if a.Comment != "" {
		return a.Comment
	}
	return "ACK! This alert was acknowledged using larm02 on %NOW%"
}

type AlertmanagerConfig struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Hidden bool   `yaml:"hidden,omitempty"`
}

// duration wraps time.Duration for yaml unmarshalling.
type duration struct{ time.Duration }

func (d *duration) UnmarshalYAML(value *yaml.Node) error {
	v, err := time.ParseDuration(value.Value)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

func (c *Config) GetRefreshInterval() time.Duration {
	if c.RefreshInterval.Duration == 0 {
		return 30 * time.Second
	}
	return c.RefreshInterval.Duration
}

// AlertmanagerURL returns the base URL for the named instance, or "" if not found.
func (c *Config) AlertmanagerURL(name string) string {
	for _, am := range c.Alertmanagers {
		if am.Name == name {
			return am.URL
		}
	}
	return ""
}

func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}
		path = filepath.Join(home, ".config", "larm02", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Alertmanagers) == 0 {
		return nil, fmt.Errorf("config must define at least one alertmanager")
	}
	for _, am := range cfg.Alertmanagers {
		if am.URL == "" {
			return nil, fmt.Errorf("alertmanager %q has no url", am.Name)
		}
	}

	if err := cfg.Theme.validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	fillThemeDefaults(&cfg.Theme)

	return &cfg, nil
}
