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

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Manage files",
}

var fileListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List files in a directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := url.Values{"projectId": {projectID}}
		if len(args) > 0 {
			q.Set("path", args[0])
		}
		q = buildListQuery(cmd, q)

		var resp struct {
			Files []map[string]interface{} `json:"files"`
		}
		if err := client.Get(cmd.Context(), "/api/files", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Files, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "SIZE", Field: "size"},
			{Header: "MODIFIED", Field: "updatedAt"},
		})
	},
}

var fileReadCmd = &cobra.Command{
	Use:   "read <file-id>",
	Short: "Read file contents to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		result, err := client.Download(cmd.Context(), "/api/files/"+args[0]+"/download")
		if err != nil {
			return err
		}
		defer result.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), result.Body)
		return err
	},
}

var fileWriteCmd = &cobra.Command{
	Use:   "write <file-id>",
	Short: "Write stdin to a file",
	Long:  "Reads content from stdin and writes it to the specified file. Use with pipes: echo 'content' | idapt file write <id>",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		content, err := io.ReadAll(f.In)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}

		body := map[string]interface{}{
			"content": string(content),
		}

		if err := client.Patch(cmd.Context(), "/api/files/"+args[0], body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File updated.")
		return nil
	},
}

var fileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"projectId": projectID,
			"name":      args[0],
		}
		if cmd.Flags().Changed("parent") {
			v, _ := cmd.Flags().GetString("parent")
			body["parentId"] = v
		}
		if cmd.Flags().Changed("content") {
			v, _ := cmd.Flags().GetString("content")
			body["content"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/files", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var fileEditCmd = &cobra.Command{
	Use:   "edit <file-id>",
	Short: "Edit a file (find and replace)",
	Long:  "Performs a find-and-replace on file content. Fetches the file, replaces old-text with new-text, and writes back.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		oldText, _ := cmd.Flags().GetString("old-text")
		newText, _ := cmd.Flags().GetString("new-text")
		if oldText == "" {
			return fmt.Errorf("--old-text is required")
		}

		// Fetch current content
		result, err := client.Download(cmd.Context(), "/api/files/"+args[0]+"/download")
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
			"content": updated,
		}
		if err := client.Patch(cmd.Context(), "/api/files/"+args[0], body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File updated.")
		return nil
	},
}

var fileDeleteCmd = &cobra.Command{
	Use:   "delete <file-id>",
	Short: "Delete a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete file %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/files/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File deleted.")
		return nil
	},
}

var fileRenameCmd = &cobra.Command{
	Use:   "rename <file-id> <new-name>",
	Short: "Rename a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{"name": args[1]}
		if err := client.Patch(cmd.Context(), "/api/files/"+args[0], body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Renamed to %s.\n", args[1])
		return nil
	},
}

var fileMoveCmd = &cobra.Command{
	Use:   "move <file-id> <target-parent-id>",
	Short: "Move a file to a different folder",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{"parentId": args[1]}
		if err := client.Patch(cmd.Context(), "/api/files/"+args[0], body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File moved.")
		return nil
	},
}

var fileMkdirCmd = &cobra.Command{
	Use:   "mkdir <name>",
	Short: "Create a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"projectId": projectID,
			"name":      args[0],
			"type":      "folder",
		}
		if cmd.Flags().Changed("parent") {
			v, _ := cmd.Flags().GetString("parent")
			body["parentId"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/files", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var fileGrepCmd = &cobra.Command{
	Use:   "grep <pattern>",
	Short: "Search file contents for a pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := url.Values{
			"projectId": {projectID},
			"q":         {args[0]},
			"type":      {"content"},
		}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "FILE", Field: "name"},
			{Header: "MATCH", Field: "snippet", Width: 80},
		})
	},
}

var fileGlobCmd = &cobra.Command{
	Use:   "glob <pattern>",
	Short: "Find files matching a glob pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := url.Values{
			"projectId": {projectID},
			"q":         {args[0]},
			"type":      {"glob"},
		}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "PATH", Field: "path"},
		})
	},
}

var fileSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search files by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := url.Values{
			"projectId": {projectID},
			"q":         {args[0]},
		}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "PATH", Field: "path"},
		})
	},
}

var fileUploadCmd = &cobra.Command{
	Use:   "upload <local-path>",
	Short: "Upload a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer file.Close()

		fi, err := file.Stat()
		if err != nil {
			return err
		}

		fields := map[string]string{
			"projectId": projectID,
		}
		if cmd.Flags().Changed("parent") {
			v, _ := cmd.Flags().GetString("parent")
			fields["parentId"] = v
		}

		resp, err := client.Upload(cmd.Context(), "/api/files", fi.Name(), file, fields)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s (%d bytes).\n", fi.Name(), fi.Size())
		return nil
	},
}

var fileDownloadCmd = &cobra.Command{
	Use:   "download <file-id> [local-path]",
	Short: "Download a file",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		result, err := client.Download(cmd.Context(), "/api/files/"+args[0]+"/download")
		if err != nil {
			return err
		}
		defer result.Body.Close()

		// Determine output path
		outPath := result.Filename
		if len(args) > 1 {
			outPath = args[1]
		}
		if outPath == "" {
			outPath = args[0]
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
	cmdutil.AddListFlags(fileListCmd)

	fileCreateCmd.Flags().String("parent", "", "Parent folder ID")
	fileCreateCmd.Flags().String("content", "", "Initial file content")

	fileEditCmd.Flags().String("old-text", "", "Text to find")
	fileEditCmd.Flags().String("new-text", "", "Replacement text")

	fileMkdirCmd.Flags().String("parent", "", "Parent folder ID")

	fileUploadCmd.Flags().String("parent", "", "Parent folder ID")

	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileReadCmd)
	fileCmd.AddCommand(fileWriteCmd)
	fileCmd.AddCommand(fileCreateCmd)
	fileCmd.AddCommand(fileEditCmd)
	fileCmd.AddCommand(fileDeleteCmd)
	fileCmd.AddCommand(fileRenameCmd)
	fileCmd.AddCommand(fileMoveCmd)
	fileCmd.AddCommand(fileMkdirCmd)
	fileCmd.AddCommand(fileGrepCmd)
	fileCmd.AddCommand(fileGlobCmd)
	fileCmd.AddCommand(fileSearchCmd)
	fileCmd.AddCommand(fileUploadCmd)
	fileCmd.AddCommand(fileDownloadCmd)
}
