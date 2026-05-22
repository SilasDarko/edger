package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Route defines a single gateway routing rule loaded from routes.yaml.
type Route struct {
	Path               string `yaml:"path"`
	Upstream           string `yaml:"upstream"`
	AuthRequired       bool   `yaml:"auth_required"`
	TimeoutMs          int    `yaml:"timeout_ms"`
	Retries            int    `yaml:"retries"`
	RateLimitPerMinute int    `yaml:"rate_limit_per_minute"`
}

// Config holds all routes loaded from the YAML config file.
type Config struct {
	Routes []Route `yaml:"routes"`
}

// Load reads and parses the YAML config file at the given path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("no routes defined")
	}
	for i, r := range cfg.Routes {
		if r.Path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}
		if r.Upstream == "" {
			return fmt.Errorf("route[%d]: upstream is required", i)
		}
		if r.TimeoutMs <= 0 {
			cfg.Routes[i].TimeoutMs = 5000
		}
		if r.RateLimitPerMinute <= 0 {
			cfg.Routes[i].RateLimitPerMinute = 60
		}
	}
	return nil
}
