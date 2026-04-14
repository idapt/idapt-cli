package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type VersionInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	SHA256      string `json:"sha256"`
}

type Updater struct {
	appURL         string
	currentVersion string
	binaryPath     string
	client         *http.Client
}

func New(appURL, currentVersion, binaryPath string) *Updater {
	return &Updater{
		appURL:         appURL,
		currentVersion: currentVersion,
		binaryPath:     binaryPath,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

func NormalizeVersion(v string) string {
	return strings.TrimPrefix(v, "cli-v")
}

func CompareVersions(a, b string) int {
	aParts := strings.SplitN(a, ".", 4)
	bParts := strings.SplitN(b, ".", 4)

	limit := 3
	if len(aParts) < limit {
		limit = len(aParts)
	}
	if len(bParts) < limit {
		limit = len(bParts)
	}

	for i := 0; i < limit; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}

	if len(aParts) > 3 && len(bParts) > 3 && aParts[3] != bParts[3] {
		return 1
	}

	return 0
}

func IsValidVersionFormat(v string) bool {
	parts := strings.SplitN(v, ".", 4)
	if len(parts) < 3 {
		return false
	}
	if len(parts[0]) != 4 {
		return false
	}
	return true
}

func (u *Updater) Check() (*VersionInfo, error) {
	url := u.appURL + "/api/cli/version"
	resp, err := u.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("check version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("version endpoint returned %d", resp.StatusCode)
	}

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parse version response: %w", err)
	}

	localVer := NormalizeVersion(u.currentVersion)
	remoteVer := NormalizeVersion(info.Version)

	if localVer == remoteVer {
		return nil, nil // up to date
	}

	if !IsValidVersionFormat(remoteVer) {
		log.Printf("update: server returned version %q which is not a valid CLI version (expected YYYY.MM.DD.HASH)", info.Version)
		return nil, nil
	}

	if CompareVersions(remoteVer, localVer) <= 0 {
		return nil, nil
	}

	return &info, nil
}

func (u *Updater) Apply(info *VersionInfo) error {
	if info.DownloadURL == "" {
		return fmt.Errorf("no download URL")
	}

	tmpPath := u.binaryPath + ".new"
	resp, err := u.client.Get(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write binary: %w", err)
	}
	tmpFile.Close()

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if info.SHA256 != "" && actualHash != info.SHA256 {
		os.Remove(tmpPath)
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", info.SHA256, actualHash)
	}

	if err := os.Rename(tmpPath, u.binaryPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	log.Printf("update: installed version %s (SHA256: %s)", info.Version, actualHash)
	return nil
}
