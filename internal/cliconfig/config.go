// Package cliconfig manages CLI-specific configuration (~/.idapt/config.json).
package cliconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds CLI-specific configuration.
type Config struct {
	APIURL         string `json:"apiUrl,omitempty"`
	DefaultProject string `json:"defaultProject,omitempty"`
	OutputFormat   string `json:"outputFormat,omitempty"`
	NoColor        bool   `json:"noColor,omitempty"`
}

// DefaultPath returns ~/.idapt/config.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".idapt", "config.json")
}

// Defaults returns a Config with default values.
func Defaults() Config {
	return Config{
		APIURL: "https://idapt.ai",
	}
}

// Load reads CLI config from the file. Returns Defaults() merged with file contents.
func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnvOverrides(cfg), nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}
	if len(data) == 0 {
		return applyEnvOverrides(cfg), nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	// Ensure default API URL if not set in file
	if cfg.APIURL == "" {
		cfg.APIURL = "https://idapt.ai"
	}

	return applyEnvOverrides(cfg), nil
}

func applyEnvOverrides(cfg Config) Config {
	if v := os.Getenv("IDAPT_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("IDAPT_PROJECT"); v != "" {
		cfg.DefaultProject = v
	}
	if v := os.Getenv("IDAPT_OUTPUT"); v != "" {
		cfg.OutputFormat = v
	}
	if os.Getenv("NO_COLOR") != "" {
		cfg.NoColor = true
	}
	return cfg
}

// Save writes the full config to disk.
func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0600)
}

// Get returns a single config value by key name.
func (c *Config) Get(key string) (string, bool) {
	switch key {
	case "apiUrl":
		return c.APIURL, true
	case "defaultProject":
		return c.DefaultProject, true
	case "outputFormat":
		return c.OutputFormat, true
	case "noColor":
		return strconv.FormatBool(c.NoColor), true
	default:
		return "", false
	}
}

// Set sets a single config value by key name.
func (c *Config) Set(key, value string) error {
	switch key {
	case "apiUrl":
		c.APIURL = value
	case "defaultProject":
		c.DefaultProject = value
	case "outputFormat":
		c.OutputFormat = value
	case "noColor":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("noColor must be true or false, got %q", value)
		}
		c.NoColor = b
	default:
		return fmt.Errorf("unknown config key %q; valid keys: %v", key, Keys())
	}
	return nil
}

// Keys returns all valid config key names.
func Keys() []string {
	return []string{"apiUrl", "defaultProject", "outputFormat", "noColor"}
}
