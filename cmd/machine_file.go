package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Manage files on a machine via SFTP",
}

var machineFileListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name> <path>",
	Short: "List files on a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		q := url.Values{"path": {args[1]}, "action": {"list"}}

		var resp struct {
			Files []map[string]interface{} `json:"files"`
		}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/sftp", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Files, []output.Column{
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "SIZE", Field: "size"},
			{Header: "PERMS", Field: "permissions"},
			{Header: "MODIFIED", Field: "modifiedAt"},
		})
	},
}

var machineFileReadCmd = &cobra.Command{
	Use:   "read <machine-id-or-name> <path>",
	Short: "Read a file from a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		q := url.Values{"path": {args[1]}}
		result, err := client.Download(cmd.Context(), "/api/machines/"+id+"/sftp/download?"+q.Encode())
		if err != nil {
			return err
		}
		defer result.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), result.Body)
		return err
	},
}

var machineFileWriteCmd = &cobra.Command{
	Use:   "write <machine-id-or-name> <path>",
	Short: "Write stdin to a file on a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"action": "write",
			"path":   args[1],
		}

		content, err := io.ReadAll(f.In)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		body["content"] = string(content)

		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/sftp", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File written.")
		return nil
	},
}

var machineFileEditCmd = &cobra.Command{
	Use:   "edit <machine-id-or-name> <path>",
	Short: "Edit a file on a machine (find and replace)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		oldText, _ := cmd.Flags().GetString("old-text")
		newText, _ := cmd.Flags().GetString("new-text")
		if oldText == "" {
			return fmt.Errorf("--old-text is required")
		}

		q := url.Values{"path": {args[1]}}
		result, err := client.Download(cmd.Context(), "/api/machines/"+id+"/sftp/download?"+q.Encode())
		if err != nil {
			return err
		}
		defer result.Body.Close()

		data, err := io.ReadAll(result.Body)
		if err != nil {
			return err
		}

		content := string(data)
		if !strings.Contains(content, oldText) {
			return fmt.Errorf("old-text not found in file")
		}
		updated := strings.Replace(content, oldText, newText, 1)

		body := map[string]interface{}{
			"action":  "write",
			"path":    args[1],
			"content": updated,
		}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/sftp", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File updated.")
		return nil
	},
}

var machineFileDeleteCmd = &cobra.Command{
	Use:   "delete <machine-id-or-name> <path>",
	Short: "Delete a file on a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete %s?", args[1])) {
				return fmt.Errorf("aborted")
			}
		}

		body := map[string]interface{}{
			"action": "delete",
			"path":   args[1],
		}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/sftp", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File deleted.")
		return nil
	},
}

var machineFileMkdirCmd = &cobra.Command{
	Use:   "mkdir <machine-id-or-name> <path>",
	Short: "Create a directory on a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"action": "mkdir",
			"path":   args[1],
		}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/sftp", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Directory created.")
		return nil
	},
}

var machineFileMoveCmd = &cobra.Command{
	Use:   "move <machine-id-or-name> <src> <dst>",
	Short: "Move/rename a file on a machine",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"action":      "rename",
			"path":        args[1],
			"destination": args[2],
		}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/sftp", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File moved.")
		return nil
	},
}

var machineFileStatCmd = &cobra.Command{
	Use:   "stat <machine-id-or-name> <path>",
	Short: "Get file info on a machine",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		q := url.Values{"path": {args[1]}, "action": {"stat"}}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/sftp", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "SIZE", Field: "size"},
			{Header: "PERMS", Field: "permissions"},
			{Header: "OWNER", Field: "owner"},
			{Header: "MODIFIED", Field: "modifiedAt"},
		})
	},
}

var machineFileGrepCmd = &cobra.Command{
	Use:   "grep <machine-id-or-name> <pattern> [path]",
	Short: "Search file contents on a machine",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		grepPath := "."
		if len(args) > 2 {
			grepPath = args[2]
		}

		grepCmd := fmt.Sprintf("grep -rn %q %s", args[1], grepPath)
		body := map[string]interface{}{
			"command": grepCmd,
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
		}
		return nil
	},
}

var machineFileFindCmd = &cobra.Command{
	Use:   "find <machine-id-or-name> <pattern> [path]",
	Short: "Find files on a machine",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		findPath := "."
		if len(args) > 2 {
			findPath = args[2]
		}

		findCmd := fmt.Sprintf("find %s -name %q", findPath, args[1])
		body := map[string]interface{}{
			"command": findCmd,
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
		}
		return nil
	},
}

var machineFileUploadCmd = &cobra.Command{
	Use:   "upload <machine-id-or-name> <local-path> <remote-path>",
	Short: "Upload a file to a machine",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		file, err := os.Open(args[1])
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer file.Close()

		fi, err := file.Stat()
		if err != nil {
			return err
		}

		fields := map[string]string{
			"path": args[2],
		}

		resp, err := client.Upload(cmd.Context(), "/api/machines/"+id+"/sftp/upload", fi.Name(), file, fields)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s to %s.\n", args[1], args[2])
		return nil
	},
}

var machineFileDownloadCmd = &cobra.Command{
	Use:   "download <machine-id-or-name> <remote-path> [local-path]",
	Short: "Download a file from a machine",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		q := url.Values{"path": {args[1]}}
		result, err := client.Download(cmd.Context(), "/api/machines/"+id+"/sftp/download?"+q.Encode())
		if err != nil {
			return err
		}
		defer result.Body.Close()

		outPath := args[1]
		if len(args) > 2 {
			outPath = args[2]
		}
		if result.Filename != "" && len(args) <= 2 {
			outPath = result.Filename
		}

		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()

		n, err := io.Copy(out, result.Body)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s (%d bytes).\n", outPath, n)
		return nil
	},
}

func init() {
	machineFileEditCmd.Flags().String("old-text", "", "Text to find")
	machineFileEditCmd.Flags().String("new-text", "", "Replacement text")

	machineFileCmd.AddCommand(machineFileListCmd)
	machineFileCmd.AddCommand(machineFileReadCmd)
	machineFileCmd.AddCommand(machineFileWriteCmd)
	machineFileCmd.AddCommand(machineFileEditCmd)
	machineFileCmd.AddCommand(machineFileDeleteCmd)
	machineFileCmd.AddCommand(machineFileMkdirCmd)
	machineFileCmd.AddCommand(machineFileMoveCmd)
	machineFileCmd.AddCommand(machineFileStatCmd)
	machineFileCmd.AddCommand(machineFileGrepCmd)
	machineFileCmd.AddCommand(machineFileFindCmd)
	machineFileCmd.AddCommand(machineFileUploadCmd)
	machineFileCmd.AddCommand(machineFileDownloadCmd)
}
