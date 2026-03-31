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

	// JWTPublicKeyPEM is the PEM-encoded EC P-256 public key for ES256 JWT verification.
	// Populated from inline config or read from JWTPublicKeyFile.
	JWTPublicKeyPEM string `json:"jwtPublicKeyPEM"`

	// JWTPublicKeyFile is the path to a PEM-encoded EC public key file.
	// If set, the file is read and its contents populate JWTPublicKeyPEM.
	JWTPublicKeyFile string `json:"jwtPublicKeyFile"`

	// JwksURL is the URL of the JWKS endpoint to fetch the ES256 public key from.
	// If empty and AppURL is set, auto-derived as AppURL + "/api/managed-machines/jwks".
	// When set, JWTPublicKeyPEM becomes optional (key is fetched dynamically).
	JwksURL string `json:"jwksUrl"`

	// MachineToken is an HMAC key for machine-level API auth (heartbeat, firewall).
	// Optional — heartbeat auth is deferred to a follow-up.
	MachineToken string `json:"machineToken"`

	// ACMEEmail is the email address for Let's Encrypt registration.
	ACMEEmail string `json:"acmeEmail"`

	// DefaultBackendPort is the port to proxy requests to by default.
	DefaultBackendPort int `json:"defaultBackendPort"`

	// CLIBinaryURL is the URL to download CLI updates from.
	CLIBinaryURL string `json:"cliBinaryUrl"`

	// APIKeyHashes is a list of pre-registered API key SHA-256 hashes (hex-encoded).
	// Loaded at startup into the APIKeyValidator. Used by cloud-init provisioning.
	APIKeyHashes []string `json:"apiKeyHashes,omitempty"`
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
	if v := os.Getenv("IDAPT_JWT_PUBLIC_KEY_PEM"); v != "" {
		cfg.JWTPublicKeyPEM = v
	}
	if v := os.Getenv("IDAPT_JWT_PUBLIC_KEY_FILE"); v != "" {
		cfg.JWTPublicKeyFile = v
	}
	if v := os.Getenv("IDAPT_JWKS_URL"); v != "" {
		cfg.JwksURL = v
	}
	if v := os.Getenv("IDAPT_MACHINE_TOKEN"); v != "" {
		cfg.MachineToken = v
	}
	if v := os.Getenv("IDAPT_ACME_EMAIL"); v != "" {
		cfg.ACMEEmail = v
	}

	// Load public key from file if specified (file takes precedence over inline PEM)
	if cfg.JWTPublicKeyFile != "" {
		pemData, err := os.ReadFile(cfg.JWTPublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read JWT public key file %s: %w", cfg.JWTPublicKeyFile, err)
		}
		cfg.JWTPublicKeyPEM = string(pemData)
	}

	// Auto-derive JwksURL from AppURL if not explicitly set.
	if cfg.JwksURL == "" && cfg.AppURL != "" {
		cfg.JwksURL = strings.TrimRight(cfg.AppURL, "/") + "/api/managed-machines/jwks"
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
	if cfg.JWTPublicKeyPEM == "" && cfg.JwksURL == "" {
		return nil, fmt.Errorf("jwtPublicKeyPEM, jwtPublicKeyFile, or jwksUrl is required")
	}
	// MachineToken is intentionally optional — heartbeat auth is deferred

	return &cfg, nil
}
