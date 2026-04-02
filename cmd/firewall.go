package cmd

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/idapt/idapt-cli/internal/config"
	"github.com/idapt/idapt-cli/internal/firewall"
	"github.com/spf13/cobra"
)

var firewallCmd = &cobra.Command{
	Use:   "firewall",
	Short: "Manage machine firewall rules",
	Long:  "View and modify the per-machine firewall rules that control which ports are publicly exposed via TLS proxy.",
}

var firewallListCmd = &cobra.Command{
	Use:   "list",
	Short: "List current firewall rules",
	RunE:  runFirewallList,
}

var firewallAddCmd = &cobra.Command{
	Use:   "add <port>[/protocol]",
	Short: "Add a public firewall rule",
	Long:  "Add a rule to publicly expose a port. Protocol defaults to tcp.\nExample: idapt firewall add 8080/tcp",
	Args:  cobra.ExactArgs(1),
	RunE:  runFirewallAdd,
}

var firewallRemoveCmd = &cobra.Command{
	Use:   "remove <port>[/protocol]",
	Short: "Remove a firewall rule",
	Long:  "Remove a public exposure rule for a port. Protocol defaults to tcp.\nExample: idapt firewall remove 8080/tcp",
	Args:  cobra.ExactArgs(1),
	RunE:  runFirewallRemove,
}

func init() {
	firewallCmd.AddCommand(firewallListCmd)
	firewallCmd.AddCommand(firewallAddCmd)
	firewallCmd.AddCommand(firewallRemoveCmd)
	rootCmd.AddCommand(firewallCmd)
}

// parsePortProtocol parses "8080" or "8080/tcp" or "8080/udp".
func parsePortProtocol(arg string) (int, string, error) {
	parts := strings.SplitN(arg, "/", 2)
	port, err := strconv.Atoi(parts[0])
	if err != nil || port < 1 || port > 65535 {
		return 0, "", fmt.Errorf("invalid port: %s (must be 1-65535)", parts[0])
	}

	protocol := "tcp"
	if len(parts) == 2 {
		protocol = strings.ToLower(parts[1])
		if protocol != "tcp" && protocol != "udp" {
			return 0, "", fmt.Errorf("invalid protocol: %s (must be tcp or udp)", parts[1])
		}
	}

	return port, protocol, nil
}

func runFirewallList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	rules, err := getLocalRules(cfg)
	if err != nil {
		return err
	}

	if len(rules) == 0 {
		fmt.Println("No firewall rules configured.")
		return nil
	}

	fmt.Printf("%-8s %-10s %s\n", "PORT", "PROTOCOL", "SOURCE")
	fmt.Println(strings.Repeat("-", 30))
	for _, r := range rules {
		fmt.Printf("%-8d %-10s %s\n", r.Port, r.Protocol, r.Source)
	}
	return nil
}

func runFirewallAdd(cmd *cobra.Command, args []string) error {
	port, protocol, err := parsePortProtocol(args[0])
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get current rules from local agent
	rules, err := getLocalRules(cfg)
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, r := range rules {
		if r.Port == port && r.Protocol == protocol {
			return fmt.Errorf("rule already exists for %d/%s", port, protocol)
		}
	}

	// Add new rule
	rules = append(rules, firewall.Rule{
		Port:     port,
		Protocol: protocol,
		Source:   "public",
	})

	// Push to app API (source of truth) which pushes back to agent
	if err := putRulesToApp(cfg, rules); err != nil {
		return err
	}

	fmt.Printf("Added rule: %d/%s (public)\n", port, protocol)
	return nil
}

func runFirewallRemove(cmd *cobra.Command, args []string) error {
	port, protocol, err := parsePortProtocol(args[0])
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	rules, err := getLocalRules(cfg)
	if err != nil {
		return err
	}

	// Find and remove rule
	found := false
	var newRules []firewall.Rule
	for _, r := range rules {
		if r.Port == port && r.Protocol == protocol {
			found = true
			continue
		}
		newRules = append(newRules, r)
	}

	if !found {
		return fmt.Errorf("no rule found for %d/%s", port, protocol)
	}

	if err := putRulesToApp(cfg, newRules); err != nil {
		return err
	}

	fmt.Printf("Removed rule: %d/%s\n", port, protocol)
	return nil
}

// getLocalRules fetches current rules from the local agent.
func getLocalRules(cfg *config.Config) ([]firewall.Rule, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	message := "GET:/api/firewall:" + timestamp
	sig := computeHMAC(message, cfg.MachineToken)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", "https://localhost/api/firewall", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(body))
	}

	var rules []firewall.Rule
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", err)
	}
	return rules, nil
}

// putRulesToApp sends rules to the app API (source of truth).
func putRulesToApp(cfg *config.Config, rules []firewall.Rule) error {
	// Convert to app API format (rules array with source field)
	type appRule struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Source   string `json:"source"`
	}
	appRules := make([]appRule, len(rules))
	for i, r := range rules {
		appRules[i] = appRule{Port: r.Port, Protocol: r.Protocol, Source: "public"}
	}

	body, err := json.Marshal(map[string]interface{}{"rules": appRules})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/managed-machines/%s/firewall", cfg.AppURL, cfg.MachineID)

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	message := "PUT:" + "/api/managed-machines/" + cfg.MachineID + "/firewall:" + timestamp
	sig := computeHMAC(message, cfg.MachineToken)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("PUT", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Machine-Signature", sig)
	req.Header.Set("X-Machine-Timestamp", timestamp)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact app API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("app API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func computeHMAC(message, key string) string {
	// key is hex-encoded, decode to binary for HMAC
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		keyBytes = []byte(key) // fallback: raw bytes if not valid hex
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
