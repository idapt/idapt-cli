package firewall

import (
	"sync"
	"testing"
)

func TestManager_SetAndGetRules(t *testing.T) {
	m := NewManager()

	rules := []Rule{
		{Port: 80, Protocol: "tcp", Source: "public"},
		{Port: 443, Protocol: "tcp", Source: "public"},
		{Port: 3000, Protocol: "tcp", Source: "public"},
	}

	m.SetRules(rules)
	got := m.GetRules()

	if len(got) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(got))
	}
	if got[0].Port != 80 {
		t.Errorf("rules[0].Port = %d, want 80", got[0].Port)
	}
	if got[2].Source != "public" {
		t.Errorf("rules[2].Source = %q, want %q", got[2].Source, "public")
	}
}

func TestManager_SetRules_ReplacesAll(t *testing.T) {
	m := NewManager()

	m.SetRules([]Rule{{Port: 80, Protocol: "tcp", Source: "public"}})
	m.SetRules([]Rule{{Port: 443, Protocol: "tcp", Source: "public"}})

	got := m.GetRules()
	if len(got) != 1 {
		t.Fatalf("len(rules) = %d, want 1 (replaced)", len(got))
	}
	if got[0].Port != 443 {
		t.Errorf("rules[0].Port = %d, want 443", got[0].Port)
	}
}

func TestManager_EmptyRules(t *testing.T) {
	m := NewManager()
	m.SetRules([]Rule{{Port: 80, Protocol: "tcp", Source: "public"}})
	m.SetRules([]Rule{}) // Clear all rules

	got := m.GetRules()
	if len(got) != 0 {
		t.Fatalf("len(rules) = %d, want 0", len(got))
	}
}

func TestManager_IsPortPublic(t *testing.T) {
	m := NewManager()
	m.SetRules([]Rule{
		{Port: 80, Protocol: "tcp", Source: "public"},
		{Port: 3000, Protocol: "tcp", Source: "public"},
	})

	if !m.IsPortPublic(80) {
		t.Error("port 80 should be public")
	}
	if !m.IsPortPublic(3000) {
		t.Error("port 3000 should be public")
	}
	if m.IsPortPublic(443) {
		t.Error("port 443 should NOT be public (no rule)")
	}
}

func TestManager_GetRules_ReturnsCopy(t *testing.T) {
	m := NewManager()
	m.SetRules([]Rule{{Port: 80, Protocol: "tcp", Source: "public"}})

	got := m.GetRules()
	got[0].Port = 9999 // Modify the returned copy

	original := m.GetRules()
	if original[0].Port != 80 {
		t.Error("modifying returned rules should not affect internal state")
	}
}

func TestManager_OnRulesChanged_Callback(t *testing.T) {
	m := NewManager()

	var callbackRules []Rule
	var callbackCalled bool
	m.SetOnRulesChanged(func(rules []Rule) {
		callbackCalled = true
		callbackRules = rules
	})

	m.SetRules([]Rule{
		{Port: 8080, Protocol: "tcp", Source: "public"},
		{Port: 3000, Protocol: "tcp", Source: "public"},
	})

	if !callbackCalled {
		t.Fatal("callback should have been called")
	}
	if len(callbackRules) != 2 {
		t.Fatalf("callback received %d rules, want 2", len(callbackRules))
	}
	if callbackRules[0].Port != 8080 {
		t.Errorf("callback rules[0].Port = %d, want 8080", callbackRules[0].Port)
	}
}

func TestManager_OnRulesChanged_ConcurrentSafe(t *testing.T) {
	m := NewManager()

	var mu sync.Mutex
	callCount := 0
	m.SetOnRulesChanged(func(rules []Rule) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			m.SetRules([]Rule{{Port: port, Protocol: "tcp", Source: "public"}})
		}(1000 + i)
	}
	wg.Wait()

	mu.Lock()
	if callCount != 10 {
		t.Errorf("callback called %d times, want 10", callCount)
	}
	mu.Unlock()
}
