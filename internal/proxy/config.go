// Package proxy manages TLS proxy configuration for exposed ports.
//
// The proxy config determines which ports get dynamic TLS listeners and
// whether each port requires authentication. Configuration is stored on the
// machine filesystem at /etc/idapt/proxy.json and is the source of truth
// (not stored in the app database).
package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// DefaultConfigPath is the default location for the proxy config file.
const DefaultConfigPath = "/etc/idapt/proxy.json"

// ProxyPort represents a single exposed port with its auth mode.
type ProxyPort struct {
	Port     int    `json:"port"`
	AuthMode string `json:"authMode"` // "authenticated" or "public"
}

// Config holds the proxy configuration — which ports are TLS-exposed.
type Config struct {
	Ports []ProxyPort `json:"ports"`
}

// ConfigManager manages the proxy config with thread-safe access
// and persistence to the filesystem.
type ConfigManager struct {
	mu       sync.RWMutex
	config   Config
	path     string
	onChange func([]ProxyPort) // Called after config changes (for listener reconciliation)
}

// NewConfigManager creates a config manager. Loads existing config from path
// if it exists, otherwise starts with an empty config.
func NewConfigManager(path string) *ConfigManager {
	cm := &ConfigManager{
		path:   path,
		config: Config{Ports: []ProxyPort{}},
	}

	// Load existing config (non-fatal on error — start empty)
	if data, err := os.ReadFile(path); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Printf("WARN: corrupt proxy config at %s, starting empty: %v", path, err)
		} else if err := validateConfig(&cfg); err != nil {
			log.Printf("WARN: invalid proxy config at %s, starting empty: %v", path, err)
		} else {
			cm.config = cfg
		}
	}

	return cm
}

// SetOnChange registers a callback invoked after config changes.
// Used by serve.go to trigger TLS listener reconciliation.
func (cm *ConfigManager) SetOnChange(fn func([]ProxyPort)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onChange = fn
}

// GetConfig returns the current proxy config.
func (cm *ConfigManager) GetConfig() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := Config{Ports: make([]ProxyPort, len(cm.config.Ports))}
	copy(result.Ports, cm.config.Ports)
	return result
}

// SetConfig replaces the entire proxy config, persists to disk,
// and notifies listeners.
func (cm *ConfigManager) SetConfig(cfg Config) error {
	if err := validateConfig(&cfg); err != nil {
		return err
	}

	cm.mu.Lock()
	cm.config = cfg
	cb := cm.onChange
	portsCopy := make([]ProxyPort, len(cfg.Ports))
	copy(portsCopy, cfg.Ports)
	cm.mu.Unlock()

	// Persist to disk (non-fatal — config is also in memory)
	if err := saveConfig(cm.path, &cfg); err != nil {
		log.Printf("WARN: failed to persist proxy config: %v", err)
	}

	// Notify listener manager
	if cb != nil {
		cb(portsCopy)
	}

	return nil
}

// GetAuthMode returns the auth mode for a given port.
// Returns "authenticated" if the port is not in the config (default).
func (cm *ConfigManager) GetAuthMode(port int) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.config.Ports {
		if p.Port == port {
			return p.AuthMode
		}
	}
	return "authenticated"
}

// IsPortPublic returns true if the port has auth mode "public".
func (cm *ConfigManager) IsPortPublic(port int) bool {
	return cm.GetAuthMode(port) == "public"
}

// TCPPorts returns the list of exposed TCP ports (for listener reconciliation).
func (cm *ConfigManager) TCPPorts() []int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ports := make([]int, 0, len(cm.config.Ports))
	for _, p := range cm.config.Ports {
		ports = append(ports, p.Port)
	}
	return ports
}

// PortCount returns the number of exposed ports.
func (cm *ConfigManager) PortCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.config.Ports)
}

// validateConfig checks that all ports are valid and auth modes are recognized.
func validateConfig(cfg *Config) error {
	seen := make(map[int]bool)
	for i, p := range cfg.Ports {
		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("port %d at index %d: must be 1-65535", p.Port, i)
		}
		if p.AuthMode != "authenticated" && p.AuthMode != "public" {
			return fmt.Errorf("port %d at index %d: authMode must be 'authenticated' or 'public'", p.Port, i)
		}
		if seen[p.Port] {
			return fmt.Errorf("duplicate port %d", p.Port)
		}
		seen[p.Port] = true
	}
	return nil
}

// saveConfig writes the config to disk atomically (temp file + rename).
func saveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}
