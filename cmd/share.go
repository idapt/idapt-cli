package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Manage shared resources",
}

var shareListCmd = &cobra.Command{
	Use:   "list",
	Short: "List resources shared with you",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Shares []map[string]interface{} `json:"shares"`
		}
		if err := client.Get(cmd.Context(), "/api/shared-with-me", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Shares, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "resourceType"},
			{Header: "NAME", Field: "resourceName"},
			{Header: "ACCESS", Field: "access"},
			{Header: "SHARED BY", Field: "sharedBy"},
		})
	},
}

var shareCreateCmd = &cobra.Command{
	Use:   "create <resource-type> <resource-id>",
	Short: "Share a resource",
	Long:  "Share a resource with another user.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		email, _ := cmd.Flags().GetString("email")
		access, _ := cmd.Flags().GetString("access")
		if email == "" {
			return fmt.Errorf("--email is required")
		}

		body := map[string]interface{}{
			"email":  email,
			"access": access,
		}

		var path string
		switch args[0] {
		case "file":
			path = fmt.Sprintf("/api/files/%s/shares", args[1])
		case "chat":
			path = fmt.Sprintf("/api/chat/%s/shares", args[1])
		case "agent":
			path = fmt.Sprintf("/api/agents/%s/shares", args[1])
		default:
			return fmt.Errorf("unsupported resource type: %s", args[0])
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), path, body, &resp); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Shared %s %s with %s.\n", args[0], args[1], email)
		return nil
	},
}

var shareDeleteCmd = &cobra.Command{
	Use:   "delete <resource-type> <resource-id> <share-id>",
	Short: "Remove a share",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var path string
		switch args[0] {
		case "file":
			path = fmt.Sprintf("/api/files/%s/shares/%s", args[1], args[2])
		case "chat":
			path = fmt.Sprintf("/api/chat/%s/shares/%s", args[1], args[2])
		case "agent":
			path = fmt.Sprintf("/api/agents/%s/shares/%s", args[1], args[2])
		default:
			return fmt.Errorf("unsupported resource type: %s", args[0])
		}

		if err := client.Delete(cmd.Context(), path); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Share removed.")
		return nil
	},
}

func init() {
	shareCreateCmd.Flags().String("email", "", "Email of user to share with")
	shareCreateCmd.Flags().String("access", "read", "Access level (read, write)")

	shareCmd.AddCommand(shareListCmd)
	shareCmd.AddCommand(shareCreateCmd)
	shareCmd.AddCommand(shareDeleteCmd)
}
