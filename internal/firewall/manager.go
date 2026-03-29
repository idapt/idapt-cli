package firewall

import (
	"sync"
)

// Rule represents a firewall rule matching the app's FirewallRule type.
// Source is always "public" — the "my-ip" option has been removed.
type Rule struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp" or "udp"
	Source   string `json:"source"`   // "public" (kept for forward-compat)
}

// Manager manages in-memory firewall rules and iptables synchronization.
type Manager struct {
	mu             sync.RWMutex
	rules          []Rule
	onRulesChanged func([]Rule)
}

// NewManager creates a new firewall manager.
func NewManager() *Manager {
	return &Manager{
		rules: make([]Rule, 0),
	}
}

// SetOnRulesChanged registers a callback invoked after rules are updated.
// Used to notify the ListenerManager to reconcile dynamic TLS listeners.
func (m *Manager) SetOnRulesChanged(fn func([]Rule)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRulesChanged = fn
}

// SetRules replaces all firewall rules atomically and notifies listeners.
func (m *Manager) SetRules(rules []Rule) {
	m.mu.Lock()
	m.rules = make([]Rule, len(rules))
	copy(m.rules, rules)
	cb := m.onRulesChanged
	rulesCopy := make([]Rule, len(rules))
	copy(rulesCopy, rules)
	m.mu.Unlock()

	// Call callback outside lock to avoid deadlocks
	if cb != nil {
		cb(rulesCopy)
	}
}

// GetRules returns a copy of the current firewall rules.
func (m *Manager) GetRules() []Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Rule, len(m.rules))
	copy(result, m.rules)
	return result
}

// IsPortPublic checks if a given port has a "public" firewall rule (no auth required).
func (m *Manager) IsPortPublic(port int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.rules {
		if r.Port == port && r.Source == "public" {
			return true
		}
	}
	return false
}
