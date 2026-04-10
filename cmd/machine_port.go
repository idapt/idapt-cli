package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machinePortCmd = &cobra.Command{
	Use:   "port",
	Short: "Manage machine port labels",
}

var machinePortListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name>",
	Short: "List open ports on a machine",
	Args:  cobra.ExactArgs(1),
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
			Ports []map[string]interface{} `json:"ports"`
		}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/ports", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Ports, []output.Column{
			{Header: "PORT", Field: "port"},
			{Header: "PROTOCOL", Field: "protocol"},
			{Header: "LABEL", Field: "label"},
			{Header: "PROCESS", Field: "process"},
		})
	},
}

var machinePortLabelCmd = &cobra.Command{
	Use:   "label <machine-id-or-name> <port> <label>",
	Short: "Set a label for a port",
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
			"port":  args[1],
			"label": args[2],
		}
		if err := client.Patch(cmd.Context(), "/api/machines/"+id+"/ports", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Port %s labeled as %s.\n", args[1], args[2])
		return nil
	},
}

func init() {
	machinePortCmd.AddCommand(machinePortListCmd)
	machinePortCmd.AddCommand(machinePortLabelCmd)
}
