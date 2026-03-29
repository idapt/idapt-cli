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
	"time"
)

// VersionInfo is the response from the app's version endpoint.
type VersionInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	SHA256      string `json:"sha256"`
}

// Updater checks for and applies agent binary updates.
type Updater struct {
	appURL         string
	currentVersion string
	binaryPath     string
	client         *http.Client
}

// New creates a new updater.
func New(appURL, currentVersion, binaryPath string) *Updater {
	return &Updater{
		appURL:         appURL,
		currentVersion: currentVersion,
		binaryPath:     binaryPath,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

// Check checks for a new version and returns the info if an update is available.
// Returns nil if already up to date.
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

	if info.Version == u.currentVersion {
		return nil, nil // up to date
	}

	// Don't downgrade
	if info.Version < u.currentVersion {
		return nil, nil
	}

	return &info, nil
}

// Apply downloads and installs the new binary.
func (u *Updater) Apply(info *VersionInfo) error {
	if info.DownloadURL == "" {
		return fmt.Errorf("no download URL")
	}

	// Download to temp file
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

	// Verify SHA256
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if info.SHA256 != "" && actualHash != info.SHA256 {
		os.Remove(tmpPath)
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", info.SHA256, actualHash)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, u.binaryPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	log.Printf("update: installed version %s (SHA256: %s)", info.Version, actualHash)
	return nil
}
