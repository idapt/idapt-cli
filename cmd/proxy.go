package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/idapt/idapt-cli/internal/config"
	"github.com/idapt/idapt-cli/internal/proxy"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage TLS proxy port exposure",
	Long:  "View and modify which ports are exposed via the TLS reverse proxy with optional authentication.",
}

var proxyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List exposed proxy ports",
	RunE:  runProxyList,
}

var proxyPublicFlag bool

var proxyExposeCmd = &cobra.Command{
	Use:   "expose <port>",
	Short: "Expose a port via TLS proxy",
	Long:  "Start a TLS listener on the given port. Default: authenticated. Use --public for unauthenticated access.",
	Args:  cobra.ExactArgs(1),
	RunE:  runProxyExpose,
}

var proxyUnexposeCmd = &cobra.Command{
	Use:   "unexpose <port>",
	Short: "Stop exposing a port",
	Args:  cobra.ExactArgs(1),
	RunE:  runProxyUnexpose,
}

func init() {
	proxyExposeCmd.Flags().BoolVar(&proxyPublicFlag, "public", false, "Allow unauthenticated access (dangerous)")
	proxyCmd.AddCommand(proxyListCmd)
	proxyCmd.AddCommand(proxyExposeCmd)
	proxyCmd.AddCommand(proxyUnexposeCmd)
	rootCmd.AddCommand(proxyCmd)
}

func runProxyList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	proxyCfg, err := getProxyConfig(cfg)
	if err != nil {
		return err
	}

	if len(proxyCfg.Ports) == 0 {
		fmt.Println("No ports exposed via proxy.")
		return nil
	}

	fmt.Printf("%-8s %s\n", "PORT", "AUTH MODE")
	fmt.Println(strings.Repeat("-", 25))
	for _, p := range proxyCfg.Ports {
		fmt.Printf("%-8d %s\n", p.Port, p.AuthMode)
	}
	return nil
}

func runProxyExpose(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %s (must be 1-65535)", args[0])
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	proxyCfg, err := getProxyConfig(cfg)
	if err != nil {
		return err
	}

	authMode := "authenticated"
	if proxyPublicFlag {
		authMode = "public"
	}

	found := false
	for i, p := range proxyCfg.Ports {
		if p.Port == port {
			proxyCfg.Ports[i].AuthMode = authMode
			found = true
			break
		}
	}
	if !found {
		proxyCfg.Ports = append(proxyCfg.Ports, proxy.ProxyPort{
			Port:     port,
			AuthMode: authMode,
		})
	}

	if err := putProxyConfig(cfg, proxyCfg); err != nil {
		return err
	}

	fmt.Printf("Exposed port %d (%s)\n", port, authMode)
	return nil
}

func runProxyUnexpose(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %s (must be 1-65535)", args[0])
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	proxyCfg, err := getProxyConfig(cfg)
	if err != nil {
		return err
	}

	found := false
	var newPorts []proxy.ProxyPort
	for _, p := range proxyCfg.Ports {
		if p.Port == port {
			found = true
			continue
		}
		newPorts = append(newPorts, p)
	}

	if !found {
		return fmt.Errorf("port %d is not exposed", port)
	}

	proxyCfg.Ports = newPorts
	if err := putProxyConfig(cfg, proxyCfg); err != nil {
		return err
	}

	fmt.Printf("Unexposed port %d\n", port)
	return nil
}

func getProxyConfig(cfg *config.Config) (*proxy.Config, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	message := "GET:/api/proxy:" + timestamp
	sig := computeHMAC(message, cfg.MachineToken)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", "https://localhost/api/proxy", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("daemon returned %d: %s", resp.StatusCode, string(body))
	}

	var proxyCfg proxy.Config
	if err := json.NewDecoder(resp.Body).Decode(&proxyCfg); err != nil {
		return nil, fmt.Errorf("failed to parse proxy config: %w", err)
	}
	return &proxyCfg, nil
}

func putProxyConfig(cfg *config.Config, proxyCfg *proxy.Config) error {
	body, err := json.Marshal(proxyCfg)
	if err != nil {
		return err
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	message := "POST:/api/proxy:" + timestamp
	sig := computeHMAC(message, cfg.MachineToken)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", "https://localhost/api/proxy", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("daemon returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
