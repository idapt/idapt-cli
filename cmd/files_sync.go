package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	ifuse "github.com/idapt/idapt-cli/internal/fuse"
	isync "github.com/idapt/idapt-cli/internal/sync"
)

var filesSyncCmd = &cobra.Command{
	Use:   "sync <project> <local-path>",
	Short: "Sync project files with a local directory",
	Long: `Bidirectional sync between idapt cloud files and a local directory.
Incremental: only downloads files that changed since last sync (via version tracking).
Use --max-files and --max-size to enforce limits (e.g., for cloud code runs).`,
	Args: cobra.ExactArgs(2),
	RunE: runFilesSync,
}

func init() {
	filesSyncCmd.Flags().String("direction", "both", "Sync direction: up, down, or both")
	filesSyncCmd.Flags().StringSlice("exclude", nil, "Patterns to exclude from sync")
	filesSyncCmd.Flags().Int("max-files", 0, "Max files to sync (0 = unlimited)")
	filesSyncCmd.Flags().Int64("max-size", 0, "Max total size in bytes (0 = unlimited)")
	filesSyncCmd.Flags().Int64("max-file-size", 0, "Max single file size in bytes (0 = unlimited)")

	fileCmd.AddCommand(filesSyncCmd)
}

type syncLimits struct {
	maxFiles    int
	maxSize     int64
	maxFileSize int64
}

type versionMap map[string]int

const versionMapFile = ".idapt-versions.json"

func runFilesSync(cmd *cobra.Command, args []string) error {
	project := args[0]
	localPath := args[1]
	direction, _ := cmd.Flags().GetString("direction")
	excludePatterns, _ := cmd.Flags().GetStringSlice("exclude")
	maxFiles, _ := cmd.Flags().GetInt("max-files")
	maxSize, _ := cmd.Flags().GetInt64("max-size")
	maxFileSize, _ := cmd.Flags().GetInt64("max-file-size")

	f := cmdutil.FactoryFromCmd(cmd)
	client, err := f.APIClient()
	if err != nil {
		return fmt.Errorf("API client: %w", err)
	}

	projectID, err := resolveProjectID(cmd, client, project)
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	fuseClient := ifuse.NewFuseAPIClient(client)
	exclusion := isync.NewExclusionEngine("", "", excludePatterns)
	limits := syncLimits{maxFiles: maxFiles, maxSize: maxSize, maxFileSize: maxFileSize}
	ctx := cmd.Context()

	switch direction {
	case "down":
		return syncDown(ctx, fuseClient, projectID, localPath, exclusion, limits, cmd.OutOrStdout())
	case "up":
		return syncUp(ctx, fuseClient, projectID, localPath, exclusion, cmd.OutOrStdout())
	case "both":
		if err := syncDown(ctx, fuseClient, projectID, localPath, exclusion, limits, cmd.OutOrStdout()); err != nil {
			return err
		}
		return syncUp(ctx, fuseClient, projectID, localPath, exclusion, cmd.OutOrStdout())
	default:
		return fmt.Errorf("invalid direction: %s (use up, down, or both)", direction)
	}
}

func syncDown(ctx context.Context, client *ifuse.FuseAPIClient, projectID, localPath string, exclusion *isync.ExclusionEngine, limits syncLimits, out io.Writer) error {
	fmt.Fprintf(out, "Syncing down from project %s to %s...\n", projectID, localPath)

	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}

	prevVersions := loadVersionMap(localPath)
	newVersions := make(versionMap)

	var allFiles []fileWithPath
	if err := collectServerFiles(ctx, client, projectID, "", "", exclusion, &allFiles); err != nil {
		return err
	}

	if limits.maxFiles > 0 && len(allFiles) > limits.maxFiles {
		return fmt.Errorf("project has %d files (limit: %d). Use a managed machine for large projects", len(allFiles), limits.maxFiles)
	}
	var totalSize int64
	for _, f := range allFiles {
		totalSize += f.entry.Size
	}
	if limits.maxSize > 0 && totalSize > limits.maxSize {
		return fmt.Errorf("project is %dMB (limit: %dMB). Use a managed machine for large projects", totalSize/(1024*1024), limits.maxSize/(1024*1024))
	}

	fileIDs := make([]string, 0, len(allFiles))
	for _, f := range allFiles {
		if !f.entry.IsFolder {
			fileIDs = append(fileIDs, f.entry.ID)
		}
	}

	serverVersions := make(map[string]int)
	if len(fileIDs) > 0 {
		var err error
		serverVersions, err = client.GetFileVersionsBatch(ctx, fileIDs)
		if err != nil {
			fmt.Fprintf(out, "  WARN: batch version check failed, doing full sync: %v\n", err)
		}
	}

	var downloaded, skipped int
	for _, f := range allFiles {
		localFilePath := filepath.Join(localPath, f.relPath)

		if f.entry.IsFolder {
			os.MkdirAll(localFilePath, 0755)
			continue
		}

		if limits.maxFileSize > 0 && f.entry.Size > limits.maxFileSize {
			fmt.Fprintf(out, "  SKIP %s (%dMB exceeds %dMB limit)\n", f.relPath, f.entry.Size/(1024*1024), limits.maxFileSize/(1024*1024))
			continue
		}

		serverV := serverVersions[f.entry.ID]
		prevV := prevVersions[f.entry.ID]
		if serverV > 0 && prevV == serverV {
			if _, err := os.Stat(localFilePath); err == nil {
				newVersions[f.entry.ID] = serverV
				skipped++
				continue
			}
		}

		reader, err := client.DownloadFile(ctx, f.entry.ID)
		if err != nil {
			fmt.Fprintf(out, "  SKIP %s (download error: %v)\n", f.relPath, err)
			continue
		}

		os.MkdirAll(filepath.Dir(localFilePath), 0755)
		outFile, err := os.Create(localFilePath)
		if err != nil {
			reader.Close()
			fmt.Fprintf(out, "  SKIP %s (create error: %v)\n", f.relPath, err)
			continue
		}

		io.Copy(outFile, reader)
		outFile.Close()
		reader.Close()

		if serverV > 0 {
			newVersions[f.entry.ID] = serverV
		} else {
			newVersions[f.entry.ID] = f.entry.Version
		}

		downloaded++
		fmt.Fprintf(out, "  ↓ %s (%d bytes)\n", f.relPath, f.entry.Size)
	}

	saveVersionMap(localPath, newVersions)

	fmt.Fprintf(out, "Done: %d downloaded, %d unchanged\n", downloaded, skipped)
	return nil
}

func syncUp(ctx context.Context, client *ifuse.FuseAPIClient, projectID, localPath string, exclusion *isync.ExclusionEngine, out io.Writer) error {
	fmt.Fprintf(out, "Syncing up from %s to project %s...\n", localPath, projectID)
	return syncUpDir(ctx, client, projectID, "", localPath, "", exclusion, out)
}

func syncUpDir(ctx context.Context, client *ifuse.FuseAPIClient, projectID, parentID, localDir, relativePath string, exclusion *isync.ExclusionEngine, out io.Writer) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", localDir, err)
	}

	for _, entry := range entries {
		relPath := filepath.Join(relativePath, entry.Name())

		if exclusion.IsExcluded(relPath) {
			continue
		}
		if entry.Name() == versionMapFile {
			continue
		}

		localFilePath := filepath.Join(localDir, entry.Name())

		if entry.IsDir() {
			folder, err := client.CreateFolder(ctx, projectID, parentID, entry.Name())
			if err != nil {
				fmt.Fprintf(out, "  SKIP dir %s (error: %v)\n", relPath, err)
				continue
			}
			if err := syncUpDir(ctx, client, projectID, folder.ID, localFilePath, relPath, exclusion, out); err != nil {
				return err
			}
			continue
		}

		content, err := os.ReadFile(localFilePath)
		if err != nil {
			fmt.Fprintf(out, "  SKIP %s (read error: %v)\n", relPath, err)
			continue
		}

		mimeType := "application/octet-stream"
		if ext := getExtension(entry.Name()); ext != "" {
			mimeType = mimeFromExt(ext)
		}

		if _, err := client.CreateFile(ctx, projectID, parentID, entry.Name(), content, mimeType); err != nil {
			fmt.Fprintf(out, "  SKIP %s (upload error: %v)\n", relPath, err)
			continue
		}

		fmt.Fprintf(out, "  ↑ %s (%d bytes)\n", relPath, len(content))
	}

	return nil
}

type fileWithPath struct {
	entry   ifuse.FileEntry
	relPath string
}

func collectServerFiles(ctx context.Context, client *ifuse.FuseAPIClient, projectID, folderID, relativePath string, exclusion *isync.ExclusionEngine, out *[]fileWithPath) error {
	files, err := client.ListFiles(ctx, projectID, folderID)
	if err != nil {
		return fmt.Errorf("list %s: %w", relativePath, err)
	}

	for _, f := range files {
		relPath := filepath.Join(relativePath, f.Name)
		if exclusion.IsExcluded(relPath) {
			continue
		}

		*out = append(*out, fileWithPath{entry: f, relPath: relPath})

		if f.IsFolder {
			if err := collectServerFiles(ctx, client, projectID, f.ID, relPath, exclusion, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadVersionMap(dir string) versionMap {
	data, err := os.ReadFile(filepath.Join(dir, versionMapFile))
	if err != nil {
		return make(versionMap)
	}
	var vm versionMap
	if err := json.Unmarshal(data, &vm); err != nil {
		return make(versionMap)
	}
	return vm
}

func saveVersionMap(dir string, vm versionMap) {
	data, _ := json.Marshal(vm)
	os.WriteFile(filepath.Join(dir, versionMapFile), data, 0644)
}

func mimeFromExt(ext string) string {
	switch ext {
	case "txt":
		return "text/plain"
	case "md":
		return "text/markdown"
	case "json":
		return "application/json"
	case "js":
		return "application/javascript"
	case "ts":
		return "application/typescript"
	case "html":
		return "text/html"
	case "css":
		return "text/css"
	default:
		return "application/octet-stream"
	}
}

func getExtension(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return ""
}
