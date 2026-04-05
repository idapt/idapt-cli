package cmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/auth"
	"github.com/idapt/idapt-cli/internal/config"
	"github.com/idapt/idapt-cli/internal/errorpages"
	"github.com/idapt/idapt-cli/internal/firewall"
	ifuse "github.com/idapt/idapt-cli/internal/fuse"
	"github.com/idapt/idapt-cli/internal/heartbeat"
	"github.com/idapt/idapt-cli/internal/listener"
	"github.com/idapt/idapt-cli/internal/network"
	"github.com/idapt/idapt-cli/internal/proxy"
	idaptTls "github.com/idapt/idapt-cli/internal/tls"
	"github.com/spf13/cobra"
)

var configPath string
var testSigCh chan os.Signal // set in test mode for /__test/signal/restart

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the idapt daemon",
	Long:  "Starts the per-machine daemon: reverse proxy, TLS termination, auth, firewall, heartbeat.",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&configPath, "config", "/etc/idapt/config.json", "Path to agent config file")
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.Printf("idapt %s starting for machine %s (domain: %s)", Version, cfg.MachineID, cfg.Domain)

	// Initialize JWT validator — prefer JWKS (dynamic key fetch) over static PEM.
	var jwtValidator *auth.JWTValidator
	var jwksFetcher *auth.JWKSFetcher // hoisted for middleware retry-on-failure
	if cfg.JwksURL != "" {
		log.Printf("Fetching JWT public key from JWKS endpoint: %s", cfg.JwksURL)
		jwksFetcher = auth.NewJWKSFetcher(cfg.JwksURL)

		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := jwksFetcher.FetchWithRetry(fetchCtx); err != nil {
			fetchCancel()
			return fmt.Errorf("failed to fetch JWKS: %w", err)
		}
		fetchCancel()

		jwtValidator, err = auth.NewJWTValidatorFromKey(jwksFetcher.GetPublicKey(), cfg.MachineID)
		if err != nil {
			return fmt.Errorf("failed to init JWT validator from JWKS key: %w", err)
		}

		// Background refresh — update the validator's key when JWKS rotates.
		jwksFetcher.SetOnRefresh(func(key *ecdsa.PublicKey) {
			jwtValidator.SetPublicKey(key)
		})

		refreshCtx, refreshCancel := context.WithCancel(context.Background())
		defer refreshCancel()
		jwksFetcher.StartRefreshLoop(refreshCtx)

		log.Printf("JWT validator initialized from JWKS (key will refresh hourly)")
	} else {
		// Fallback: static PEM key from config file.
		jwtValidator, err = auth.NewJWTValidator(cfg.JWTPublicKeyPEM, cfg.MachineID)
		if err != nil {
			return fmt.Errorf("failed to init JWT validator: %w", err)
		}
		log.Printf("JWT validator initialized from static PEM key")
	}

	fwManager := firewall.NewManager()
	reverseProxy := proxy.New(cfg.DefaultBackendPort)
	pages := errorpages.New(cfg.Domain, cfg.AppURL)

	// Proxy config manager — loads from /etc/idapt/proxy.json, source of truth for TLS exposure
	proxyCfg := proxy.NewConfigManager(proxy.DefaultConfigPath)

	// Auth middleware — uses proxy config for per-port auth mode (not firewall)
	authMiddleware := auth.NewMiddleware(jwtValidator, proxyCfg, pages, cfg.Domain, cfg.AppURL)
	if jwksFetcher != nil {
		authMiddleware.SetJWKSFetcher(jwksFetcher)
	}

	// HTTP mux
	mux := http.NewServeMux()

	// Management API (machine-level HMAC auth, not user JWT)
	mux.HandleFunc("POST /api/firewall", firewall.NewHandler(fwManager, cfg.MachineToken))
	mux.HandleFunc("GET /api/firewall", firewall.NewGetHandler(fwManager, cfg.MachineToken))
	mux.HandleFunc("GET /api/firewall/iptables", firewall.NewIptablesReadHandler(cfg.MachineToken))
	mux.HandleFunc("GET /api/proxy", proxy.NewGetHandler(proxyCfg, cfg.MachineToken))
	mux.HandleFunc("POST /api/proxy", proxy.NewPostHandler(proxyCfg, cfg.MachineToken))
	// fuseMountsRef is captured by the health endpoint closure below.
	// Set after MountManager is initialized (further down in this function).
	var fuseMountsRef *ifuse.MountManager

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mountCount := 0
		if fuseMountsRef != nil {
			mountCount = len(fuseMountsRef.ActiveMounts())
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"status":     "ok",
			"version":    Version,
			"proxyPorts": proxyCfg.PortCount(),
			"fuseMounts": mountCount,
		})
		w.Write(resp)
	})

	// ACME challenge path — always open, no auth
	mux.HandleFunc("GET /.well-known/acme-challenge/", func(w http.ResponseWriter, r *http.Request) {
		// certmagic handles this internally via its HTTP handler
		http.NotFound(w, r)
	})

	// Test-mode management endpoints (only when IDAPT_TEST_MODE=1)
	if os.Getenv("IDAPT_TEST_MODE") == "1" {
		testSigCh = make(chan os.Signal, 1)
		mux.HandleFunc("POST /__test/signal/restart", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
			go func() {
				time.Sleep(100 * time.Millisecond)
				testSigCh <- syscall.SIGUSR1
			}()
		})
		mux.HandleFunc("POST /__test/validate-jwt", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Token string `json:"token"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			claims, err := jwtValidator.Validate(body.Token)
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"valid": false, "error": err.Error(),
				})
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"valid": true, "claims": claims,
				})
			}
		})
		mux.HandleFunc("POST /__test/set-machine-id", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				MachineID string `json:"machineId"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.MachineID != "" {
				jwtValidator.SetMachineID(body.MachineID)
				log.Printf("TEST MODE: machine ID updated to %s", body.MachineID)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		})
		mux.HandleFunc("POST /__test/exec", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Command string `json:"command"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			out, err := exec.Command("sh", "-c", body.Command).CombinedOutput()
			w.Header().Set("Content-Type", "application/json")
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"output": string(out), "error": errStr,
			})
		})
		mux.HandleFunc("POST /__test/block-app", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Block bool `json:"block"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			// Uses iptables OUTPUT chain to block/unblock traffic to app server
			appHost := cfg.AppURL
			var cmd *exec.Cmd
			if body.Block {
				cmd = exec.Command("sh", "-c", fmt.Sprintf("iptables -A OUTPUT -d %s -j DROP 2>/dev/null || true", appHost))
			} else {
				cmd = exec.Command("sh", "-c", fmt.Sprintf("iptables -D OUTPUT -d %s -j DROP 2>/dev/null || true", appHost))
			}
			cmd.Run()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		})
		log.Printf("TEST MODE: /__test/* endpoints enabled")
	}

	// All other requests go through auth + proxy
	mux.HandleFunc("/", authMiddleware.Wrap(reverseProxy.ServeHTTP))

	// TLS configuration: CertMagic (ACME) with self-signed fallback.
	// Always generate a self-signed cert so HTTPS works immediately at startup,
	// even before CertMagic obtains a real cert (DNS may not be propagated yet).
	selfSignedConfig, err := idaptTls.SelfSignedConfig(cfg.Domain)
	if err != nil {
		return fmt.Errorf("failed to create self-signed cert: %w", err)
	}
	selfSignedGetCert := selfSignedConfig.GetCertificate
	if selfSignedGetCert == nil && len(selfSignedConfig.Certificates) > 0 {
		// SelfSignedConfig uses Certificates field, wrap it as GetCertificate
		selfCert := selfSignedConfig.Certificates[0]
		selfSignedGetCert = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &selfCert, nil
		}
	}

	var tlsConfig *tls.Config
	var acmeHandler http.Handler
	acmeTlsConfig, acmeH, acmeErr := idaptTls.SetupCertMagic(cfg.Domain, cfg.ACMEEmail)
	if acmeErr != nil {
		log.Printf("WARN: ACME setup failed, using self-signed only: %v", acmeErr)
		tlsConfig = selfSignedConfig
	} else {
		acmeHandler = acmeH
		// Compose: try CertMagic first, fall back to self-signed if no cert available
		acmeGetCert := acmeTlsConfig.GetCertificate
		tlsConfig = acmeTlsConfig
		tlsConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if acmeGetCert != nil {
				cert, err := acmeGetCert(hello)
				if cert != nil && err == nil {
					return cert, nil
				}
			}
			// CertMagic has no cert yet — use self-signed fallback
			if selfSignedGetCert != nil {
				return selfSignedGetCert(hello)
			}
			return nil, fmt.Errorf("no certificate available")
		}
		log.Printf("TLS: CertMagic + self-signed fallback configured for %s", cfg.Domain)
	}

	// HTTPS server (port 443, or IDAPT_HTTPS_PORT override for non-root containers)
	httpsPort := getEnvOrDefault("IDAPT_HTTPS_PORT", "443")
	httpsServer := &http.Server{
		Addr:      ":" + httpsPort,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	// HTTP server (port 80) — ACME challenges + redirect
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Let ACME handler try first
		if acmeHandler != nil {
			acmeHandler.ServeHTTP(w, r)
			return
		}
		// Redirect to HTTPS
		target := "https://" + r.Host + r.RequestURI
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	// HTTP server (port 80, or IDAPT_HTTP_PORT override for non-root containers)
	httpPort := getEnvOrDefault("IDAPT_HTTP_PORT", "80")
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: httpHandler,
	}

	// Start heartbeat (only if machineToken is configured — heartbeat auth is deferred)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if cfg.MachineToken != "" {
		hb := heartbeat.New(cfg.AppURL, cfg.MachineID, cfg.MachineToken, Version)
		go hb.Start(ctx)
	} else {
		log.Printf("heartbeat: disabled (machineToken not configured)")
	}

	// FUSE mount manager — auto-mount configured projects
	fuseMM := ifuse.NewMountManager()
	fuseMountsRef = fuseMM // expose to health endpoint closure
	if len(cfg.Mounts) > 0 {
		// Build API client for FUSE (reuses machine token auth)
		fuseAPIClient, fuseErr := buildFuseAPIClient(cfg)
		if fuseErr != nil {
			log.Printf("fuse-mount: disabled (API client error: %v)", fuseErr)
		} else {
			for _, m := range cfg.Mounts {
				maxCache := int64(m.MaxCacheSizeGB) * 1024 * 1024 * 1024
				if maxCache == 0 {
					maxCache = 10 * 1024 * 1024 * 1024 // default 10GB
				}
				mountCfg := ifuse.MountConfig{
					ProjectID:       m.ProjectID,
					MountPoint:      m.MountPoint,
					CacheDir:        m.CacheDir,
					MaxCacheSize:    maxCache,
					ExcludePatterns: m.ExcludePatterns,
				}
				if err := fuseMM.Mount(ctx, mountCfg, fuseAPIClient); err != nil {
					log.Printf("fuse-mount: failed to mount %s at %s: %v", m.ProjectID, m.MountPoint, err)
				}
			}
		}
	} else {
		log.Printf("fuse-mount: no mounts configured")
	}

	// Start servers
	errCh := make(chan error, 10)

	// Dynamic multi-port TLS listener manager
	lm := listener.New(mux, tlsConfig, cfg.Domain, errCh)

	// Detect public IP for:
	// 1. Binding dynamic listeners to public IP only (avoids conflict with 127.0.0.1 services)
	// 2. Self-signed cert for direct IP access
	if publicIP := network.GetPublicIP(); publicIP != "" {
		lm.SetPublicIP(publicIP)
		if err := lm.SetIPCert(publicIP, cfg.Domain); err != nil {
			log.Printf("WARN: Failed to generate IP cert: %v", err)
		}
	}

	// Proxy config drives TLS listeners (not firewall — firewall is just iptables).
	// When proxy config changes, reconcile dynamic TLS listeners.
	proxyCfg.SetOnChange(func(ports []proxy.ProxyPort) {
		var tcpPorts []int
		for _, p := range ports {
			tcpPorts = append(tcpPorts, p.Port)
		}
		lm.Reconcile(tcpPorts)
	})

	// Initial reconciliation from config file (loaded on startup)
	lm.Reconcile(proxyCfg.TCPPorts())

	go func() {
		log.Printf("HTTPS server listening on :%s", httpsPort)
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTPS server: %w", err)
		}
	}()

	go func() {
		log.Printf("HTTP server listening on :%s", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Graceful shutdown / seamless restart
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGHUP)
	// In test mode, also listen on the test signal channel
	if testSigCh != nil {
		go func() {
			for sig := range testSigCh {
				sigCh <- sig
			}
		}()
	}

	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGHUP {
				// Reload proxy config from disk (used when config is pushed via SSH)
				log.Printf("Received SIGHUP — reloading proxy config from disk")
				if err := proxyCfg.ReloadFromDisk(); err != nil {
					log.Printf("WARN: proxy config reload failed: %v", err)
				} else {
					log.Printf("Proxy config reloaded from disk: %d ports", proxyCfg.PortCount())
				}
				continue // keep running, don't shut down
			}
			if sig == syscall.SIGUSR1 {
				// Seamless restart: graceful shutdown then exec() the new binary.
				log.Printf("Received SIGUSR1 — restarting with updated binary...")

				cancel() // stop heartbeat

				drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
				fuseMM.Shutdown(drainCtx) // flush + unmount FUSE
				lm.Shutdown(drainCtx)
				httpsServer.Shutdown(drainCtx)
				httpServer.Shutdown(drainCtx)
				drainCancel()

				exe, err := os.Executable()
				if err != nil {
					log.Fatalf("Failed to resolve executable path for restart: %v", err)
				}
				log.Printf("Exec'ing new binary: %s %v", exe, os.Args)
				if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
					log.Fatalf("Exec failed (systemd will restart with new binary): %v", err)
				}
				// unreachable — Exec replaces the process
			}
			log.Printf("Received %s, shutting down gracefully...", sig)
		case err := <-errCh:
			log.Printf("Server error: %v, shutting down...", err)
		}
		break // exit the for loop to proceed with shutdown
	}

	cancel() // stop heartbeat

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	fuseMM.Shutdown(shutdownCtx) // unmount FUSE first (flush dirty files)
	lm.Shutdown(shutdownCtx)     // stop dynamic listeners
	httpsServer.Shutdown(shutdownCtx)
	httpServer.Shutdown(shutdownCtx)

	log.Printf("idapt stopped")
	return nil
}

// buildFuseAPIClient creates an API client for FUSE mount operations.
func buildFuseAPIClient(cfg *config.Config) (*ifuse.FuseAPIClient, error) {
	apiClient, err := api.NewClient(api.ClientConfig{
		BaseURL: cfg.AppURL,
		APIKey:  cfg.MachineToken, // uses machine token for auth
	})
	if err != nil {
		return nil, err
	}
	return ifuse.NewFuseAPIClient(apiClient), nil
}
