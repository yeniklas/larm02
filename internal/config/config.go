package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Alertmanagers   []AlertmanagerConfig `yaml:"alertmanagers"`
	RefreshInterval duration             `yaml:"refresh_interval"`
}

type AlertmanagerConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
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

	return &cfg, nil
}
