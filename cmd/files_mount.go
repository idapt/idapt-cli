package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	ifuse "github.com/idapt/idapt-cli/internal/fuse"
)

var filesMountCmd = &cobra.Command{
	Use:   "mount <project> <mountpoint>",
	Short: "Mount project files as a local filesystem",
	Long:  "Mount idapt cloud files as a FUSE filesystem. Files are synced via OCC.",
	Args:  cobra.ExactArgs(2),
	RunE:  runFilesMount,
}

var filesUnmountCmd = &cobra.Command{
	Use:   "unmount <mountpoint>",
	Short: "Unmount a FUSE filesystem",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesUnmount,
}

var mountManager *ifuse.MountManager

func init() {
	filesMountCmd.Flags().StringSlice("exclude", nil, "Patterns to exclude from sync (comma-separated)")
	filesMountCmd.Flags().String("cache-dir", "", "Directory for local file cache")
	filesMountCmd.Flags().Int64("cache-size", 10*1024*1024*1024, "Maximum cache size in bytes (default 10GB)")

	// Register under the existing "file" parent command
	fileCmd.AddCommand(filesMountCmd)
	fileCmd.AddCommand(filesUnmountCmd)
}

func getMountManager() *ifuse.MountManager {
	if mountManager == nil {
		mountManager = ifuse.NewMountManager()
	}
	return mountManager
}

func runFilesMount(cmd *cobra.Command, args []string) error {
	project := args[0]
	mountPoint := args[1]

	f := cmdutil.FactoryFromCmd(cmd)
	client, err := f.APIClient()
	if err != nil {
		return fmt.Errorf("API client: %w", err)
	}

	// Resolve project to ID
	projectID, err := resolveProjectID(cmd, client, project)
	if err != nil {
		// Hint at API key issues — common when using sudo
		if strings.Contains(err.Error(), "Missing permission") || strings.Contains(err.Error(), "not accessible") {
			return fmt.Errorf("resolve project: %w\n\nHint: Your API key may lack 'project:read' permission.\nIf using sudo, pass --api-key explicitly or use 'sudo -E' to preserve IDAPT_API_KEY.", err)
		}
		return fmt.Errorf("resolve project: %w", err)
	}

	excludePatterns, _ := cmd.Flags().GetStringSlice("exclude")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	cacheSize, _ := cmd.Flags().GetInt64("cache-size")

	cfg := ifuse.MountConfig{
		ProjectID:       projectID,
		MountPoint:      mountPoint,
		CacheDir:        cacheDir,
		MaxCacheSize:    cacheSize,
		ExcludePatterns: excludePatterns,
	}

	fuseClient := ifuse.NewFuseAPIClient(client)
	mm := getMountManager()

	if err := mm.Mount(cmd.Context(), cfg, fuseClient); err != nil {
		// Hint at stale mount recovery
		if strings.Contains(err.Error(), "Transport endpoint") || strings.Contains(err.Error(), "fusermount") {
			return fmt.Errorf("mount: %w\n\nHint: A stale FUSE mount may exist. Run: fusermount3 -u %s", err, mountPoint)
		}
		return fmt.Errorf("mount: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mounted %s at %s\n", project, mountPoint)
	fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to unmount.\n")

	// Block until interrupt (for interactive CLI use)
	<-cmd.Context().Done()

	if err := mm.Unmount(mountPoint); err != nil {
		return fmt.Errorf("unmount: %w", err)
	}

	return nil
}

func runFilesUnmount(cmd *cobra.Command, args []string) error {
	mountPoint := args[0]
	mm := getMountManager()

	if err := mm.Unmount(mountPoint); err != nil {
		return fmt.Errorf("unmount: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Unmounted %s\n", mountPoint)
	return nil
}

// resolveProjectID resolves a project name/slug/ID to a project ID.
func resolveProjectID(cmd *cobra.Command, client *api.Client, project string) (string, error) {
	// If it looks like a UUID, use directly
	if len(project) == 36 && strings.Count(project, "-") == 4 {
		return project, nil
	}

	// Otherwise resolve via API
	var resp struct {
		Projects []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"projects"`
	}
	if err := client.Get(cmd.Context(), "/api/projects", nil, &resp); err != nil {
		return "", err
	}

	for _, p := range resp.Projects {
		if p.Slug == project || p.ID == project {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project %q not found", project)
}
