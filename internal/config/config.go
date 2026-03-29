package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds the agent configuration.
type Config struct {
	// MachineID is the managed machine UUID in the app database.
	MachineID string `json:"machineId"`

	// AppURL is the base URL of the idapt app (e.g., "https://idapt.ai").
	AppURL string `json:"appUrl"`

	// Domain is the machine's subdomain (e.g., "my-machine.idapt.app").
	Domain string `json:"domain"`

	// JWTSecret is the base secret for HKDF key derivation (shared with the app).
	JWTSecret string `json:"jwtSecret"`

	// MachineToken is an HMAC key for machine-level API auth (heartbeat, firewall).
	MachineToken string `json:"machineToken"`

	// ACMEEmail is the email address for Let's Encrypt registration.
	ACMEEmail string `json:"acmeEmail"`

	// DefaultBackendPort is the port to proxy requests to by default.
	DefaultBackendPort int `json:"defaultBackendPort"`

	// CLIBinaryURL is the URL to download CLI updates from.
	CLIBinaryURL string `json:"cliBinaryUrl"`
}

// Load reads the config from a JSON file, with env var overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	// Environment variable overrides
	if v := os.Getenv("IDAPT_MACHINE_ID"); v != "" {
		cfg.MachineID = v
	}
	if v := os.Getenv("IDAPT_APP_URL"); v != "" {
		cfg.AppURL = v
	}
	if v := os.Getenv("IDAPT_DOMAIN"); v != "" {
		cfg.Domain = v
	}
	if v := os.Getenv("IDAPT_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("IDAPT_MACHINE_TOKEN"); v != "" {
		cfg.MachineToken = v
	}
	if v := os.Getenv("IDAPT_ACME_EMAIL"); v != "" {
		cfg.ACMEEmail = v
	}

	// Defaults
	if cfg.DefaultBackendPort == 0 {
		cfg.DefaultBackendPort = 80
	}
	if cfg.ACMEEmail == "" {
		cfg.ACMEEmail = "machines@idapt.ai"
	}

	// Validation
	if cfg.MachineID == "" {
		return nil, fmt.Errorf("machineId is required")
	}
	if cfg.AppURL == "" {
		return nil, fmt.Errorf("appUrl is required")
	}
	if cfg.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if strings.Contains(cfg.Domain, "*") {
		return nil, fmt.Errorf("domain must be a specific subdomain, not a wildcard: %s", cfg.Domain)
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("jwtSecret is required")
	}
	if cfg.MachineToken == "" {
		return nil, fmt.Errorf("machineToken is required")
	}

	return &cfg, nil
}
