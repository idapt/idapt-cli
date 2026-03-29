package firewall

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testMachineToken = "test-machine-token-secret"

func signRequest(method, path string) (string, string) {
	timestamp := "1700000000"
	message := method + ":" + path + ":" + timestamp
	mac := hmac.New(sha256.New, []byte(testMachineToken))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))
	return sig, timestamp
}

func TestFirewallHandler_POST_ValidRules(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	rules := []Rule{
		{Port: 80, Protocol: "tcp", Source: "public"},
		{Port: 443, Protocol: "tcp", Source: "public"},
	}
	body, _ := json.Marshal(rules)

	req := httptest.NewRequest("POST", "/api/firewall", bytes.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	got := mgr.GetRules()
	if len(got) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(got))
	}
}

func TestFirewallHandler_POST_NoAuth(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	body := `[{"port":80,"protocol":"tcp","source":"public"}]`
	req := httptest.NewRequest("POST", "/api/firewall", strings.NewReader(body))

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestFirewallHandler_POST_InvalidHMAC(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	body := `[{"port":80,"protocol":"tcp","source":"public"}]`
	req := httptest.NewRequest("POST", "/api/firewall", strings.NewReader(body))
	req.Header.Set("X-Machine-Signature", "deadbeef")
	req.Header.Set("X-Machine-Timestamp", "1700000000")

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestFirewallHandler_POST_InvalidJSON(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	req := httptest.NewRequest("POST", "/api/firewall", strings.NewReader("{invalid}"))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFirewallHandler_POST_InvalidPort(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	rules := []Rule{{Port: 0, Protocol: "tcp", Source: "public"}}
	body, _ := json.Marshal(rules)

	req := httptest.NewRequest("POST", "/api/firewall", bytes.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFirewallHandler_POST_PortTooHigh(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	rules := []Rule{{Port: 65536, Protocol: "tcp", Source: "public"}}
	body, _ := json.Marshal(rules)

	req := httptest.NewRequest("POST", "/api/firewall", bytes.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFirewallHandler_POST_InvalidProtocol(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	rules := []Rule{{Port: 80, Protocol: "icmp", Source: "public"}}
	body, _ := json.Marshal(rules)

	req := httptest.NewRequest("POST", "/api/firewall", bytes.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFirewallHandler_POST_TooManyRules(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	rules := make([]Rule, 101)
	for i := range rules {
		rules[i] = Rule{Port: i + 1, Protocol: "tcp", Source: "public"}
	}
	body, _ := json.Marshal(rules)

	req := httptest.NewRequest("POST", "/api/firewall", bytes.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFirewallHandler_POST_EmptyRules(t *testing.T) {
	mgr := NewManager()
	mgr.SetRules([]Rule{{Port: 80, Protocol: "tcp", Source: "public"}})

	handler := NewHandler(mgr, testMachineToken)

	body := `[]`
	req := httptest.NewRequest("POST", "/api/firewall", strings.NewReader(body))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got := mgr.GetRules()
	if len(got) != 0 {
		t.Errorf("rules should be empty after setting []")
	}
}

func TestFirewallHandler_POST_BodyTooLarge(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr, testMachineToken)

	largeBody := strings.Repeat("x", maxBodySize+1)
	req := httptest.NewRequest("POST", "/api/firewall", strings.NewReader(largeBody))
	sig, ts := signRequest("POST", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestFirewallHandler_GET(t *testing.T) {
	mgr := NewManager()
	mgr.SetRules([]Rule{{Port: 80, Protocol: "tcp", Source: "public"}})

	handler := NewGetHandler(mgr, testMachineToken)

	req := httptest.NewRequest("GET", "/api/firewall", nil)
	sig, ts := signRequest("GET", "/api/firewall")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", ts)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var rules []Rule
	if err := json.NewDecoder(w.Body).Decode(&rules); err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Port != 80 {
		t.Errorf("unexpected rules: %+v", rules)
	}
}

func TestFirewallHandler_GET_NoAuth(t *testing.T) {
	mgr := NewManager()
	handler := NewGetHandler(mgr, testMachineToken)

	req := httptest.NewRequest("GET", "/api/firewall", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// Suppress unused import warning
var _ = fmt.Sprint
