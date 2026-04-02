package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/idapt/idapt-cli/internal/config"
	"github.com/idapt/idapt-cli/internal/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and apply updates",
	Long:  "Checks for a new version of the idapt daemon, downloads it, and signals the running daemon to seamlessly restart.",
	RunE: func(cmd *cobra.Command, args []string) error {
		appURL := os.Getenv("IDAPT_APP_URL")
		if appURL == "" {
			// Try to read appUrl from daemon config
			if cfg, err := config.Load(configPath); err == nil {
				appURL = cfg.AppURL
			}
		}
		if appURL == "" {
			appURL = "https://idapt.ai"
		}

		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}

		updater := update.New(appURL, Version, binaryPath)

		info, err := updater.Check()
		if err != nil {
			return fmt.Errorf("check for updates: %w", err)
		}

		if info == nil {
			fmt.Println("Already up to date.")
			return nil
		}

		fmt.Printf("Update available: %s → %s\n", Version, info.Version)

		if err := updater.Apply(info); err != nil {
			return fmt.Errorf("apply update: %w", err)
		}

		// Signal the running daemon to seamlessly restart with the new binary.
		// The daemon catches SIGUSR1, gracefully drains connections, then exec()s
		// the new binary in-place (same PID, near-zero downtime).
		if pid := findServePID(); pid > 0 {
			if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
				fmt.Printf("Updated to %s. Failed to signal daemon (PID %d): %v — restart manually.\n",
					info.Version, pid, err)
			} else {
				fmt.Printf("Updated to %s. Daemon restarting seamlessly (PID %d).\n", info.Version, pid)
			}
		} else {
			fmt.Printf("Updated to %s. No running daemon found — restart manually to apply.\n", info.Version)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// findServePID locates the running `idapt serve` daemon's PID.
// Tries systemctl first (most reliable on managed machines), then /proc scan.
func findServePID() int {
	// Method 1: systemctl (works on machines with systemd service installed)
	if pid := findPIDViaSystemctl(); pid > 0 {
		return pid
	}
	// Method 2: scan /proc for idapt serve process (works everywhere on Linux)
	if pid := findPIDViaProcScan(); pid > 0 {
		return pid
	}
	return 0
}

// findPIDViaSystemctl queries systemd for the idapt service's main PID.
func findPIDViaSystemctl() int {
	out, err := exec.Command("systemctl", "show", "idapt.service", "-p", "MainPID", "--value").Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// findPIDViaProcScan scans /proc/*/cmdline for an `idapt serve` process.
// Skips our own PID (the update command).
func findPIDViaProcScan() int {
	myPID := os.Getpid()
	entries, err := filepath.Glob("/proc/[0-9]*/cmdline")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		// cmdline is NUL-separated: ["idapt", "serve", "--config", ...]
		cmdline := string(data)
		if !strings.Contains(cmdline, "idapt") || !strings.Contains(cmdline, "serve") {
			continue
		}
		// Extract PID from path: /proc/12345/cmdline
		parts := strings.Split(entry, "/")
		if len(parts) < 3 {
			continue
		}
		pid, err := strconv.Atoi(parts[2])
		if err != nil || pid <= 0 || pid == myPID {
			continue
		}
		return pid
	}
	return 0
}
