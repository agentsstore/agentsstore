package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Listen  string `yaml:"listen"`
	DataDir string `yaml:"data_dir"`
	BaseURL string `yaml:"base_url"`
}

type Source struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"` // "git" or "http"
	URL     string `yaml:"url"`
	Ref     string `yaml:"ref"` // git only
	Enabled bool   `yaml:"enabled"`
}

type Config struct {
	Server  ServerConfig `yaml:"server"`
	Sources []Source     `yaml:"sources"`
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func (s Source) Validate() error {
	if !nameRe.MatchString(s.Name) {
		return fmt.Errorf("invalid name %q (must match %s)", s.Name, nameRe)
	}
	if s.Type != "git" && s.Type != "http" {
		return fmt.Errorf("type must be git or http, got %q", s.Type)
	}
	if s.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

func (c *Config) Validate() error {
	seen := map[string]bool{}
	for i, s := range c.Sources {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("sources[%d]: %w", i, err)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate source name %q", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Server.DataDir == "" {
		c.Server.DataDir = "./data"
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
