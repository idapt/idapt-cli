package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage account settings",
}

var settingsGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get settings (all or a specific key)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/settings", nil, &resp); err != nil {
			return err
		}

		if len(args) > 0 {
			key := args[0]
			val, ok := resp[key]
			if !ok {
				return fmt.Errorf("unknown setting: %s", key)
			}
			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "THEME", Field: "theme"},
			{Header: "DEFAULT MODEL", Field: "defaultModel"},
			{Header: "SLUG", Field: "slug"},
		})
	},
}

var settingsSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a setting value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			args[0]: args[1],
		}

		if err := client.Patch(cmd.Context(), "/api/settings", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Setting %s updated.\n", args[0])
		return nil
	},
}

func init() {
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)
}
