package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineEnvVarCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"env-var"},
	Short:   "Manage environment variables for machine users",
}

var machineEnvVarListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name> <username>",
	Short: "List environment variables for a machine user (names only, no values)",
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

		var resp struct {
			EnvVars []map[string]interface{} `json:"envVars"`
		}
		path := "/api/machines/" + id + "/users/" + url.PathEscape(args[1]) + "/env"
		if err := client.Get(cmd.Context(), path, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.EnvVars, []output.Column{
			{Header: "NAME", Field: "secretName"},
			{Header: "ID", Field: "id"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var machineEnvVarSetCmd = &cobra.Command{
	Use:   "set <machine-id-or-name> <username>",
	Short: "Set an environment variable from a credential file or inline value",
	Long: `Set an environment variable for a machine user.

The value can come from:
  --secret-id   Existing credential file ID (recommended for security)
  --name/--value  Inline name+value (creates a credential file automatically)

Examples:
  idapt machine env set my-server root --name DATABASE_URL --value "postgres://..."
  idapt machine env set my-server deploy --secret-id abc-123-def`,
	Args: cobra.ExactArgs(2),
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

		body := map[string]interface{}{}

		secretID, _ := cmd.Flags().GetString("secret-id")
		name, _ := cmd.Flags().GetString("name")
		value, _ := cmd.Flags().GetString("value")

		if secretID != "" {
			body["secretId"] = secretID
		} else if name != "" && value != "" {
			body["name"] = name
			body["value"] = value
		} else {
			return fmt.Errorf("provide either --secret-id or both --name and --value")
		}

		var resp map[string]interface{}
		path := "/api/machines/" + id + "/users/" + url.PathEscape(args[1]) + "/env"
		if err := client.Post(cmd.Context(), path, body, &resp); err != nil {
			return err
		}

		if name != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Environment variable $%s set.\n", name)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Environment variable set from credential file.")
		}
		return nil
	},
}

var machineEnvVarDeleteCmd = &cobra.Command{
	Use:   "delete <machine-id-or-name> <username> <binding-id>",
	Short: "Delete an environment variable binding",
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

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, "Delete this environment variable?") {
				return fmt.Errorf("aborted")
			}
		}

		path := "/api/machines/" + id + "/users/" + url.PathEscape(args[1]) + "/env/" + args[2]
		if err := client.Delete(cmd.Context(), path); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Environment variable removed.")
		return nil
	},
}

var machineEnvVarSetupCmd = &cobra.Command{
	Use:   "setup <machine-id-or-name> <username>",
	Short: "Setup shell integration for environment variables (~/.idapt-env + .bashrc)",
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

		var resp map[string]interface{}
		path := "/api/machines/" + id + "/users/" + url.PathEscape(args[1]) + "/env/setup"
		if err := client.Post(cmd.Context(), path, nil, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Shell integration configured.")
		return nil
	},
}

var machineEnvVarSyncCmd = &cobra.Command{
	Use:   "sync <machine-id-or-name> <username>",
	Short: "Check or repair sync status between DB and ~/.idapt-env",
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

		repair, _ := cmd.Flags().GetBool("repair")
		path := "/api/machines/" + id + "/users/" + url.PathEscape(args[1]) + "/env/sync"

		if repair {
			var resp map[string]interface{}
			if err := client.Post(cmd.Context(), path, nil, &resp); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Environment variables re-synced.")
			return nil
		}

		var resp struct {
			IsInSync   bool                     `json:"isInSync"`
			Entries    []map[string]interface{}  `json:"entries"`
			ExtraOnDisk []string                `json:"extraOnDisk"`
		}
		if err := client.Get(cmd.Context(), path, nil, &resp); err != nil {
			return err
		}

		if resp.IsInSync {
			fmt.Fprintln(cmd.OutOrStdout(), "In sync.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Out of sync. Use --repair to fix.")
			formatter := f.Formatter()
			return formatter.WriteList(resp.Entries, []output.Column{
				{Header: "NAME", Field: "secretName"},
				{Header: "ON DISK", Field: "presentOnDisk"},
			})
		}
		return nil
	},
}

func init() {
	machineEnvVarSetCmd.Flags().String("secret-id", "", "Credential file ID to bind")
	machineEnvVarSetCmd.Flags().String("name", "", "Environment variable name")
	machineEnvVarSetCmd.Flags().String("value", "", "Environment variable value (creates credential file)")

	machineEnvVarSyncCmd.Flags().Bool("repair", false, "Repair sync by rewriting ~/.idapt-env from DB state")

	machineEnvVarCmd.AddCommand(machineEnvVarListCmd)
	machineEnvVarCmd.AddCommand(machineEnvVarSetCmd)
	machineEnvVarCmd.AddCommand(machineEnvVarDeleteCmd)
	machineEnvVarCmd.AddCommand(machineEnvVarSetupCmd)
	machineEnvVarCmd.AddCommand(machineEnvVarSyncCmd)
}
