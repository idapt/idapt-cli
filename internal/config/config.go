package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	MachineID string `json:"machineId"`

	AppURL string `json:"appUrl"`

	Domain string `json:"domain"`

	JWTPublicKeyPEM string `json:"jwtPublicKeyPEM"`

	JWTPublicKeyFile string `json:"jwtPublicKeyFile"`

	JwksURL string `json:"jwksUrl"`

	MachineToken string `json:"machineToken"`

	ACMEEmail string `json:"acmeEmail"`

	DefaultBackendPort int `json:"defaultBackendPort"`

	CLIBinaryURL string `json:"cliBinaryUrl"`

	Mounts []MountEntry `json:"mounts,omitempty"`
}

type MountEntry struct {
	ProjectID       string   `json:"projectId"`
	MountPoint      string   `json:"mountPoint"`
	CacheDir        string   `json:"cacheDir,omitempty"`
	MaxCacheSizeGB  int      `json:"maxCacheSizeGB,omitempty"` // default 10
	ExcludePatterns []string `json:"excludePatterns,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

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

	if cfg.JWTPublicKeyFile != "" {
		pemData, err := os.ReadFile(cfg.JWTPublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read JWT public key file %s: %w", cfg.JWTPublicKeyFile, err)
		}
		cfg.JWTPublicKeyPEM = string(pemData)
	}

	if cfg.JwksURL == "" && cfg.AppURL != "" {
		cfg.JwksURL = strings.TrimRight(cfg.AppURL, "/") + "/api/managed-machines/jwks"
	}

	if cfg.DefaultBackendPort == 0 {
		cfg.DefaultBackendPort = 80
	}
	if cfg.ACMEEmail == "" {
		cfg.ACMEEmail = "machines@idapt.ai"
	}

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

	return &cfg, nil
}
