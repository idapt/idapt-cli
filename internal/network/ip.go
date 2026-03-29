// Package network provides public IP detection for managed machines.
package network

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// GetPublicIP tries to detect the machine's public IPv4 address.
// Tries cloud metadata endpoints first (EC2 IMDSv2, Hetzner), then
// falls back to a public IP echo service. Returns "" on failure.
func GetPublicIP() string {
	// 1. EC2 IMDSv2 (requires token)
	if ip := getEC2PublicIP(); ip != "" {
		return ip
	}
	// 2. Hetzner metadata
	if ip := fetchURL("http://169.254.169.254/hetzner/v1/metadata/public-ipv4", nil, 2*time.Second); ip != "" {
		return ip
	}
	// 3. Public IP echo service
	if ip := fetchURL("https://checkip.amazonaws.com", nil, 5*time.Second); ip != "" {
		return ip
	}
	return ""
}

// getEC2PublicIP uses IMDSv2 (token-based) to retrieve the public IPv4.
func getEC2PublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}

	// Step 1: Get IMDSv2 token
	tokenReq, err := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		return ""
	}
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return ""
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != 200 {
		return ""
	}

	tokenBytes, err := io.ReadAll(io.LimitReader(tokenResp.Body, 256))
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return ""
	}

	// Step 2: Fetch public IP using token
	headers := map[string]string{"X-aws-ec2-metadata-token": token}
	return fetchURL("http://169.254.169.254/latest/meta-data/public-ipv4", headers, 2*time.Second)
}

// fetchURL performs a GET request and returns the trimmed body or "".
func fetchURL(url string, headers map[string]string, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}

	ip := strings.TrimSpace(string(body))
	// Basic sanity: should look like an IP (contains dots, no spaces)
	if !strings.Contains(ip, ".") || strings.Contains(ip, " ") {
		return ""
	}
	return ip
}
