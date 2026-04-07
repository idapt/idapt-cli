package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineExecCmd = &cobra.Command{
	Use:   "exec <id-or-name> <command>",
	Short: "Execute a command on a machine",
	Args:  cobra.MinimumNArgs(2),
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

		command := args[1]
		if len(args) > 2 {
			command = ""
			for i := 1; i < len(args); i++ {
				if i > 1 {
					command += " "
				}
				command += args[i]
			}
		}

		user, _ := cmd.Flags().GetString("user")
		timeout, _ := cmd.Flags().GetInt("timeout")

		body := map[string]interface{}{
			"command": command,
		}
		if user != "" {
			body["user"] = user
		}
		if timeout > 0 {
			body["timeout"] = timeout
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "EXIT CODE", Field: "exitCode"},
			{Header: "OUTPUT", Field: "output"},
			{Header: "STDERR", Field: "stderr"},
		})
	},
}

func init() {
	machineExecCmd.Flags().String("user", "", "Run as user")
	machineExecCmd.Flags().Int("timeout", 0, "Timeout in seconds")
}
