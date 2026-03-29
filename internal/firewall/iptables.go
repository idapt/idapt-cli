package firewall

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

const chainName = "IDAPT-FIREWALL"

// protectedPorts are always allowed regardless of user rules
var protectedPorts = []int{22, 80, 443}

// ApplyRules applies firewall rules via iptables.
// Creates/flushes the IDAPT-FIREWALL chain, adds protected ports,
// then adds user-defined rules.
func ApplyRules(rules []Rule) error {
	// Ensure chain exists
	exec.Command("iptables", "-N", chainName).Run() // ignore error if exists

	// Flush chain
	if err := run("iptables", "-F", chainName); err != nil {
		return fmt.Errorf("flush chain: %w", err)
	}

	// Ensure chain is referenced from INPUT
	// Check if jump rule exists, add if not
	out, _ := exec.Command("iptables", "-S", "INPUT").Output()
	if !strings.Contains(string(out), chainName) {
		run("iptables", "-I", "INPUT", "-j", chainName)
	}

	// Protected ports — always ACCEPT
	for _, port := range protectedPorts {
		if err := run("iptables", "-A", chainName, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil {
			log.Printf("iptables: failed to add protected port %d: %v", port, err)
		}
	}

	// Allow established/related connections
	run("iptables", "-A", chainName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")

	// Allow loopback
	run("iptables", "-A", chainName, "-i", "lo", "-j", "ACCEPT")

	// User-defined rules (all rules are source "public" — no source restriction)
	for _, rule := range rules {
		args := []string{"-A", chainName, "-p", rule.Protocol, "--dport", fmt.Sprintf("%d", rule.Port), "-j", "ACCEPT"}
		if err := run("iptables", args...); err != nil {
			log.Printf("iptables: failed to add rule port %d: %v", rule.Port, err)
		}
	}

	// Default DROP for chain (only for NEW connections to non-matched ports)
	run("iptables", "-A", chainName, "-m", "conntrack", "--ctstate", "NEW", "-j", "DROP")

	return nil
}

// ClearRules removes all rules from the IDAPT-FIREWALL chain.
func ClearRules() error {
	exec.Command("iptables", "-F", chainName).Run()
	return nil
}

// ReadRules reads the current iptables rules from the IDAPT-FIREWALL chain.
// Parses `iptables -S IDAPT-FIREWALL` output and returns user-defined ACCEPT rules.
func ReadRules() ([]Rule, error) {
	out, err := exec.Command("iptables", "-S", chainName).Output()
	if err != nil {
		return nil, fmt.Errorf("iptables -S %s: %w", chainName, err)
	}

	var rules []Rule
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Only parse ACCEPT rules with --dport (user-defined port rules)
		if !strings.Contains(line, "-j ACCEPT") || !strings.Contains(line, "--dport") {
			continue
		}

		rule := Rule{Source: "public"}

		// Extract protocol: -p tcp or -p udp
		if strings.Contains(line, "-p tcp") {
			rule.Protocol = "tcp"
		} else if strings.Contains(line, "-p udp") {
			rule.Protocol = "udp"
		} else {
			continue
		}

		// Extract port: --dport 8080
		dportIdx := strings.Index(line, "--dport ")
		if dportIdx == -1 {
			continue
		}
		portStr := ""
		rest := line[dportIdx+8:]
		for _, ch := range rest {
			if ch >= '0' && ch <= '9' {
				portStr += string(ch)
			} else {
				break
			}
		}
		port := 0
		for _, ch := range portStr {
			port = port*10 + int(ch-'0')
		}
		if port < 1 || port > 65535 {
			continue
		}
		rule.Port = port

		rules = append(rules, rule)
	}

	return rules, nil
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s (%w)", cmd, strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return nil
}
