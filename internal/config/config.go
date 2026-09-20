// Package config loads and validates the list of services to monitor.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default values applied when a service does not set them.
const (
	DefaultMethod         = "GET"
	DefaultExpectedStatus = 200
	DefaultTimeout        = 5 * time.Second
)

// Service is a single endpoint to check.
type Service struct {
	Name           string        `yaml:"name"`
	URL            string        `yaml:"url"`
	Method         string        `yaml:"method"`
	ExpectedStatus int           `yaml:"expected_status"`
	Timeout        time.Duration `yaml:"timeout"`
}

// Config is the top-level structure of the config file.
type Config struct {
	Services []Service `yaml:"services"`
}

// Load reads a YAML config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return Parse(data)
}

// Parse decodes YAML, fills in defaults, and validates every service.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if len(cfg.Services) == 0 {
		return nil, errors.New("config has no services")
	}

	for i := range cfg.Services {
		s := &cfg.Services[i]
		applyDefaults(s)
		if err := validate(s); err != nil {
			return nil, fmt.Errorf("service #%d: %w", i+1, err)
		}
	}
	return &cfg, nil
}

func applyDefaults(s *Service) {
	if s.Method == "" {
		s.Method = DefaultMethod
	}
	s.Method = strings.ToUpper(s.Method)
	if s.ExpectedStatus == 0 {
		s.ExpectedStatus = DefaultExpectedStatus
	}
	if s.Timeout == 0 {
		s.Timeout = DefaultTimeout
	}
	if s.Name == "" {
		s.Name = s.URL
	}
}

func validate(s *Service) error {
	if s.URL == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid url %q (must start with http:// or https://)", s.URL)
	}
	if s.Timeout < 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}
