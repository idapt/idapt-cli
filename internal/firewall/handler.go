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

func NewHandler(mgr *Manager, machineToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.ValidateMachineHMAC(r, machineToken); err != nil {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
		if err != nil {
			http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
			return
		}
		if len(body) > maxBodySize {
			http.Error(w, `{"error":"body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}

		var rules []Rule
		if err := json.Unmarshal(body, &rules); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}

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

		if len(rules) > 100 {
			http.Error(w, `{"error":"too many rules (max 100)"}`, http.StatusBadRequest)
			return
		}

		mgr.SetRules(rules)

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
