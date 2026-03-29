package firewall

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/idapt/idapt-cli/internal/auth"
)

const maxBodySize = 1 << 20 // 1MB

// NewHandler creates an HTTP handler for POST /api/firewall.
// Accepts a JSON array of firewall rules, authenticated via HMAC.
func NewHandler(mgr *Manager, machineToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate HMAC auth
		if err := auth.ValidateMachineHMAC(r, machineToken); err != nil {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		// Read body with size limit
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
		if err != nil {
			http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
			return
		}
		if len(body) > maxBodySize {
			http.Error(w, `{"error":"body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}

		// Parse rules
		var rules []Rule
		if err := json.Unmarshal(body, &rules); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		// Validate rules
		for i, rule := range rules {
			if rule.Port < 1 || rule.Port > 65535 {
				http.Error(w, fmt.Sprintf(`{"error":"rule %d: port must be 1-65535"}`, i), http.StatusBadRequest)
				return
			}
			if rule.Protocol != "tcp" && rule.Protocol != "udp" {
				http.Error(w, fmt.Sprintf(`{"error":"rule %d: protocol must be tcp or udp"}`, i), http.StatusBadRequest)
				return
			}
		}

		// Limit total rules
		if len(rules) > 100 {
			http.Error(w, `{"error":"too many rules (max 100)"}`, http.StatusBadRequest)
			return
		}

		// Apply rules
		mgr.SetRules(rules)

		// Apply iptables rules (best-effort, log errors)
		if err := ApplyRules(rules); err != nil {
			log.Printf("iptables apply failed (rules stored in memory): %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accepted": true,
			"count":    len(rules),
		})
	}
}

// NewGetHandler creates an HTTP handler for GET /api/firewall.
func NewGetHandler(mgr *Manager, machineToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.ValidateMachineHMAC(r, machineToken); err != nil {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		rules := mgr.GetRules()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

// NewIptablesReadHandler creates a handler for GET /api/firewall/iptables.
// Reads the actual iptables state from the IDAPT-FIREWALL chain (just-in-time).
func NewIptablesReadHandler(machineToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.ValidateMachineHMAC(r, machineToken); err != nil {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		rules, err := ReadRules()
		if err != nil {
			log.Printf("Failed to read iptables rules: %v", err)
			http.Error(w, fmt.Sprintf(`{"error":"failed to read iptables: %s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}
