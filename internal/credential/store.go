// Package credential manages stored authentication credentials.
package credential

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds stored authentication credentials.
type Credentials struct {
	APIKey string `json:"apiKey,omitempty"`
}

// DefaultPath returns ~/.idapt/credentials.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".idapt", "credentials.json")
}

// Load reads credentials from the file. Returns zero-value Credentials
// (not an error) if the file does not exist.
func Load(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, nil
		}
		return Credentials{}, fmt.Errorf("reading credentials: %w", err)
	}
	if len(data) == 0 {
		return Credentials{}, nil
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("parsing credentials: %w", err)
	}
	return creds, nil
}

// Save writes credentials to the file. Creates directory if needed.
func Save(path string, creds Credentials) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

// Clear removes the credentials file.
func Clear(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing credentials: %w", err)
	}
	return nil
}
