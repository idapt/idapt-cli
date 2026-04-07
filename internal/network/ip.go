package network

import (
	"io"
	"net/http"
	"strings"
	"time"
)

func GetPublicIP() string {
	if ip := getEC2PublicIP(); ip != "" {
		return ip
	}
	if ip := fetchURL("http://169.254.169.254/hetzner/v1/metadata/public-ipv4", nil, 2*time.Second); ip != "" {
		return ip
	}
	if ip := fetchURL("https://checkip.amazonaws.com", nil, 5*time.Second); ip != "" {
		return ip
	}
	return ""
}

func getEC2PublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}

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

	headers := map[string]string{"X-aws-ec2-metadata-token": token}
	return fetchURL("http://169.254.169.254/latest/meta-data/public-ipv4", headers, 2*time.Second)
}

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
	if !strings.Contains(ip, ".") || strings.Contains(ip, " ") {
		return ""
	}
	return ip
}
