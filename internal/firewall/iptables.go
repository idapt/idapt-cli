package firewall

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

const chainName = "IDAPT-FIREWALL"

var protectedPorts = []int{22, 80, 443}

func ApplyRules(rules []Rule) error {
	exec.Command("iptables", "-N", chainName).Run() // ignore error if exists

	if err := run("iptables", "-F", chainName); err != nil {
		return fmt.Errorf("flush chain: %w", err)
	}

	out, _ := exec.Command("iptables", "-S", "INPUT").Output()
	if !strings.Contains(string(out), chainName) {
		run("iptables", "-I", "INPUT", "-j", chainName)
	}

	for _, port := range protectedPorts {
		if err := run("iptables", "-A", chainName, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil {
			log.Printf("iptables: failed to add protected port %d: %v", port, err)
		}
	}

	run("iptables", "-A", chainName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")

	run("iptables", "-A", chainName, "-i", "lo", "-j", "ACCEPT")

	for _, rule := range rules {
		args := []string{"-A", chainName, "-p", rule.Protocol, "--dport", fmt.Sprintf("%d", rule.Port), "-j", "ACCEPT"}
		if err := run("iptables", args...); err != nil {
			log.Printf("iptables: failed to add rule port %d: %v", rule.Port, err)
		}
	}

	run("iptables", "-A", chainName, "-m", "conntrack", "--ctstate", "NEW", "-j", "DROP")

	return nil
}

func ClearRules() error {
	exec.Command("iptables", "-F", chainName).Run()
	return nil
}

func ReadRules() ([]Rule, error) {
	out, err := exec.Command("iptables", "-S", chainName).Output()
	if err != nil {
		return nil, fmt.Errorf("iptables -S %s: %w", chainName, err)
	}

	var rules []Rule
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "-j ACCEPT") || !strings.Contains(line, "--dport") {
			continue
		}

		rule := Rule{Source: "public"}

		if strings.Contains(line, "-p tcp") {
			rule.Protocol = "tcp"
		} else if strings.Contains(line, "-p udp") {
			rule.Protocol = "udp"
		} else {
			continue
		}

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
