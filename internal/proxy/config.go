package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const DefaultConfigPath = "/etc/idapt/proxy.json"

type ProxyPort struct {
	Port     int    `json:"port"`
	AuthMode string `json:"authMode"` // "authenticated" or "public"
}

type Config struct {
	Ports []ProxyPort `json:"ports"`
}

type ConfigManager struct {
	mu       sync.RWMutex
	config   Config
	path     string
	onChange func([]ProxyPort) // Called after config changes (for listener reconciliation)
}

func NewConfigManager(path string) *ConfigManager {
	cm := &ConfigManager{
		path:   path,
		config: Config{Ports: []ProxyPort{}},
	}

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

func (cm *ConfigManager) SetOnChange(fn func([]ProxyPort)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onChange = fn
}

func (cm *ConfigManager) GetConfig() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := Config{Ports: make([]ProxyPort, len(cm.config.Ports))}
	copy(result.Ports, cm.config.Ports)
	return result
}

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

	if err := saveConfig(cm.path, &cfg); err != nil {
		log.Printf("WARN: failed to persist proxy config: %v", err)
	}

	if cb != nil {
		cb(portsCopy)
	}

	return nil
}

func (cm *ConfigManager) ReloadFromDisk() error {
	data, err := os.ReadFile(cm.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.SetConfig(Config{Ports: []ProxyPort{}})
		}
		return fmt.Errorf("read proxy config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse proxy config: %w", err)
	}

	return cm.SetConfig(cfg)
}

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

func (cm *ConfigManager) IsPortPublic(port int) bool {
	return cm.GetAuthMode(port) == "public"
}

func (cm *ConfigManager) TCPPorts() []int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ports := make([]int, 0, len(cm.config.Ports))
	for _, p := range cm.config.Ports {
		ports = append(ports, p.Port)
	}
	return ports
}

func (cm *ConfigManager) PortCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.config.Ports)
}

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
