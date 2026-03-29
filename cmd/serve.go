package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/idapt/idapt-cli/internal/auth"
	"github.com/idapt/idapt-cli/internal/config"
	"github.com/idapt/idapt-cli/internal/errorpages"
	"github.com/idapt/idapt-cli/internal/firewall"
	"github.com/idapt/idapt-cli/internal/heartbeat"
	"github.com/idapt/idapt-cli/internal/listener"
	"github.com/idapt/idapt-cli/internal/network"
	"github.com/idapt/idapt-cli/internal/proxy"
	idaptTls "github.com/idapt/idapt-cli/internal/tls"
	"github.com/spf13/cobra"
)

var configPath string

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

	// Initialize components
	jwtValidator, err := auth.NewJWTValidator(cfg.JWTSecret, cfg.MachineID)
	if err != nil {
		return fmt.Errorf("failed to init JWT validator: %w", err)
	}

	apiKeyValidator := auth.NewAPIKeyValidator()
	fwManager := firewall.NewManager()
	reverseProxy := proxy.New(cfg.DefaultBackendPort)
	pages := errorpages.New(cfg.Domain, cfg.AppURL)

	// Proxy config manager — loads from /etc/idapt/proxy.json, source of truth for TLS exposure
	proxyCfg := proxy.NewConfigManager(proxy.DefaultConfigPath)

	// Auth middleware — uses proxy config for per-port auth mode (not firewall)
	authMiddleware := auth.NewMiddleware(jwtValidator, apiKeyValidator, proxyCfg, pages)

	// HTTP mux
	mux := http.NewServeMux()

	// Management API (machine-level HMAC auth, not user JWT)
	mux.HandleFunc("POST /api/firewall", firewall.NewHandler(fwManager, cfg.MachineToken))
	mux.HandleFunc("GET /api/firewall", firewall.NewGetHandler(fwManager, cfg.MachineToken))
	mux.HandleFunc("GET /api/firewall/iptables", firewall.NewIptablesReadHandler(cfg.MachineToken))
	mux.HandleFunc("GET /api/proxy", proxy.NewGetHandler(proxyCfg, cfg.MachineToken))
	mux.HandleFunc("POST /api/proxy", proxy.NewPostHandler(proxyCfg, cfg.MachineToken))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"` + Version + `","proxyPorts":` + fmt.Sprintf("%d", proxyCfg.PortCount()) + `}`))
	})

	// ACME challenge path — always open, no auth
	mux.HandleFunc("GET /.well-known/acme-challenge/", func(w http.ResponseWriter, r *http.Request) {
		// certmagic handles this internally via its HTTP handler
		http.NotFound(w, r)
	})

	// All other requests go through auth + proxy
	mux.HandleFunc("/", authMiddleware.Wrap(reverseProxy.ServeHTTP))

	// TLS configuration
	tlsConfig, acmeHandler, err := idaptTls.SetupCertMagic(cfg.Domain, cfg.ACMEEmail)
	if err != nil {
		log.Printf("WARN: ACME setup failed, using self-signed: %v", err)
		tlsConfig, err = idaptTls.SelfSignedConfig(cfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to create self-signed cert: %w", err)
		}
	}

	// HTTPS server (port 443)
	httpsServer := &http.Server{
		Addr:      ":443",
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

	httpServer := &http.Server{
		Addr:    ":80",
		Handler: httpHandler,
	}

	// Start heartbeat
	hb := heartbeat.New(cfg.AppURL, cfg.MachineID, cfg.MachineToken, Version)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hb.Start(ctx)

	// Start servers
	errCh := make(chan error, 10)

	// Dynamic multi-port TLS listener manager
	lm := listener.New(mux, tlsConfig, cfg.Domain, errCh)

	// Generate self-signed cert for direct IP access (non-fatal)
	if publicIP := network.GetPublicIP(); publicIP != "" {
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
		log.Printf("HTTPS server listening on :443")
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTPS server: %w", err)
		}
	}()

	go func() {
		log.Printf("HTTP server listening on :80")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Graceful shutdown / seamless restart
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1)

	select {
	case sig := <-sigCh:
		if sig == syscall.SIGUSR1 {
			// Seamless restart: graceful shutdown then exec() the new binary.
			// The update command sends SIGUSR1 after replacing the binary on disk.
			// syscall.Exec replaces this process in-place (same PID, no systemd restart).
			log.Printf("Received SIGUSR1 — restarting with updated binary...")

			cancel() // stop heartbeat

			drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	cancel() // stop heartbeat

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	lm.Shutdown(shutdownCtx) // stop dynamic listeners first
	httpsServer.Shutdown(shutdownCtx)
	httpServer.Shutdown(shutdownCtx)

	log.Printf("idapt stopped")
	return nil
}
