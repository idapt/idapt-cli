//go:build daemontest

package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// getMachineState fetches the managed machine state from the app admin API.
func getMachineState(t *testing.T) map[string]interface{} {
	t.Helper()

	body := map[string]interface{}{
		"action":           "get-machine",
		"managedMachineId": machineID,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/admin/test/managed-machine-state", appURL), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-test-secret", testSecret)
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Machine state request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Machine state returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to parse machine state: %v", err)
	}

	// The response wraps the machine data in {"managedMachine": {...}}
	if mm, ok := result["managedMachine"].(map[string]interface{}); ok {
		return mm
	}
	return result
}

func TestHeartbeatReceivedOnStartup(t *testing.T) {
	t.Skip("heartbeat tests require matching machineId between daemon and app")
	state := getMachineState(t)

	lastActivityRaw, ok := state["lastActivityAt"]
	if !ok {
		t.Skip("Machine state does not include lastActivityAt — skipping heartbeat test")
	}

	lastActivityStr, ok := lastActivityRaw.(string)
	if !ok {
		t.Skipf("lastActivityAt is not a string: %T", lastActivityRaw)
	}

	lastActivity, err := time.Parse(time.RFC3339, lastActivityStr)
	if err != nil {
		// Try parsing with milliseconds
		lastActivity, err = time.Parse(time.RFC3339Nano, lastActivityStr)
		if err != nil {
			t.Fatalf("Failed to parse lastActivityAt %q: %v", lastActivityStr, err)
		}
	}

	// Heartbeat should have been sent on daemon startup — within the last 60s
	elapsed := time.Since(lastActivity)
	if elapsed > 60*time.Second {
		t.Errorf("lastActivityAt is %v ago (expected within 60s) — heartbeat may not be working", elapsed.Round(time.Second))
	}
}

func TestHeartbeatRecurring(t *testing.T) {
	t.Skip("heartbeat tests require matching machineId between daemon and app")
	if testing.Short() {
		t.Skip("Skipping recurring heartbeat test in short mode")
	}

	// Record initial state
	state1 := getMachineState(t)
	lastActivity1Raw, ok := state1["lastActivityAt"]
	if !ok {
		t.Skip("Machine state does not include lastActivityAt")
	}
	lastActivity1Str, _ := lastActivity1Raw.(string)
	lastActivity1, err := time.Parse(time.RFC3339Nano, lastActivity1Str)
	if err != nil {
		lastActivity1, err = time.Parse(time.RFC3339, lastActivity1Str)
		if err != nil {
			t.Fatalf("Failed to parse initial lastActivityAt: %v", err)
		}
	}

	// Wait for at least one heartbeat interval (30s) plus buffer
	t.Log("Waiting 35s for recurring heartbeat...")
	time.Sleep(35 * time.Second)

	// Check again — should be more recent
	state2 := getMachineState(t)
	lastActivity2Raw, ok := state2["lastActivityAt"]
	if !ok {
		t.Fatal("Machine state missing lastActivityAt on second check")
	}
	lastActivity2Str, _ := lastActivity2Raw.(string)
	lastActivity2, err := time.Parse(time.RFC3339Nano, lastActivity2Str)
	if err != nil {
		lastActivity2, err = time.Parse(time.RFC3339, lastActivity2Str)
		if err != nil {
			t.Fatalf("Failed to parse second lastActivityAt: %v", err)
		}
	}

	if !lastActivity2.After(lastActivity1) {
		t.Errorf("lastActivityAt did not advance: before=%v, after=%v", lastActivity1, lastActivity2)
	}
}
