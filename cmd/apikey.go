package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var apikeyCmd = &cobra.Command{
	Use:     "apikey",
	Aliases: []string{"api-key"},
	Short:   "Manage API keys",
}

var apikeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		if err := client.Get(cmd.Context(), "/api/api-keys", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Keys, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "PREFIX", Field: "prefix"},
			{Header: "PERMISSIONS", Field: "permissions"},
			{Header: "CREATED", Field: "createdAt"},
			{Header: "LAST USED", Field: "lastUsedAt"},
		})
	},
}

var apikeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		body := map[string]interface{}{
			"name": name,
		}

		if cmd.Flags().Changed("permissions") {
			v, _ := cmd.Flags().GetString("permissions")
			body["permissions"] = []map[string]interface{}{
				{"resource": "*", "access": v},
			}
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/api-keys", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "KEY", Field: "key"},
		})
	},
}

var apikeyDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete API key %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/api-keys/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "API key deleted.")
		return nil
	},
}

func init() {
	apikeyCreateCmd.Flags().String("name", "", "API key name")
	apikeyCreateCmd.Flags().String("permissions", "", "Permissions (read, write, admin)")

	apikeyCmd.AddCommand(apikeyListCmd)
	apikeyCmd.AddCommand(apikeyCreateCmd)
	apikeyCmd.AddCommand(apikeyDeleteCmd)
}
