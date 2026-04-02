package listener

import (
	"net/http"
	"testing"
)

func TestBindAddr_WithPublicIP(t *testing.T) {
	lm := &ListenerManager{
		listeners: make(map[int]*http.Server),
		publicIP:  "203.0.113.42",
	}

	addr := lm.bindAddr(8080)
	expected := "203.0.113.42:8080"
	if addr != expected {
		t.Errorf("bindAddr(8080) = %q, want %q", addr, expected)
	}
}

func TestBindAddr_WithoutPublicIP(t *testing.T) {
	lm := &ListenerManager{
		listeners: make(map[int]*http.Server),
	}

	addr := lm.bindAddr(8080)
	expected := ":8080"
	if addr != expected {
		t.Errorf("bindAddr(8080) = %q, want %q", addr, expected)
	}
}

func TestSetPublicIP(t *testing.T) {
	lm := &ListenerManager{
		listeners: make(map[int]*http.Server),
	}

	// Initially empty
	if lm.publicIP != "" {
		t.Errorf("initial publicIP = %q, want empty", lm.publicIP)
	}

	// Set IP
	lm.SetPublicIP("10.0.0.1")
	if lm.publicIP != "10.0.0.1" {
		t.Errorf("publicIP after SetPublicIP = %q, want %q", lm.publicIP, "10.0.0.1")
	}

	// Verify bindAddr uses it
	addr := lm.bindAddr(3001)
	if addr != "10.0.0.1:3001" {
		t.Errorf("bindAddr(3001) after SetPublicIP = %q, want %q", addr, "10.0.0.1:3001")
	}
}

func TestBindAddr_DifferentPorts(t *testing.T) {
	lm := &ListenerManager{
		listeners: make(map[int]*http.Server),
		publicIP:  "192.168.1.100",
	}

	tests := []struct {
		port     int
		expected string
	}{
		{80, "192.168.1.100:80"},
		{443, "192.168.1.100:443"},
		{3000, "192.168.1.100:3000"},
		{8080, "192.168.1.100:8080"},
		{65535, "192.168.1.100:65535"},
	}

	for _, tt := range tests {
		addr := lm.bindAddr(tt.port)
		if addr != tt.expected {
			t.Errorf("bindAddr(%d) = %q, want %q", tt.port, addr, tt.expected)
		}
	}
}
