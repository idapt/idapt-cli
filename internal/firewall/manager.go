package firewall

import (
	"sync"
)

type Rule struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp" or "udp"
	Source   string `json:"source"`   // "public" (kept for forward-compat)
}

type Manager struct {
	mu             sync.RWMutex
	rules          []Rule
	onRulesChanged func([]Rule)
}

func NewManager() *Manager {
	return &Manager{
		rules: make([]Rule, 0),
	}
}

func (m *Manager) SetOnRulesChanged(fn func([]Rule)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRulesChanged = fn
}

func (m *Manager) SetRules(rules []Rule) {
	m.mu.Lock()
	m.rules = make([]Rule, len(rules))
	copy(m.rules, rules)
	cb := m.onRulesChanged
	rulesCopy := make([]Rule, len(rules))
	copy(rulesCopy, rules)
	m.mu.Unlock()

	if cb != nil {
		cb(rulesCopy)
	}
}

func (m *Manager) GetRules() []Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Rule, len(m.rules))
	copy(result, m.rules)
	return result
}

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
