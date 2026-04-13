package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/idapt/idapt-cli/internal/auth"
)

const maxBodySize = 1 << 20 // 1MB

func NewGetHandler(cm *ConfigManager, machineToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.ValidateMachineHMAC(r, machineToken); err != nil {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		cfg := cm.GetConfig()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

func NewPostHandler(cm *ConfigManager, machineToken string) http.HandlerFunc {
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

		var cfg Config
		if err := json.Unmarshal(body, &cfg); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		if err := cm.SetConfig(cfg); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		log.Printf("Proxy config updated: %d ports exposed", len(cfg.Ports))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accepted": true,
			"count":    len(cfg.Ports),
		})
	}
}
