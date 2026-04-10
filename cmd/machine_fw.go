package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineFwCmd = &cobra.Command{
	Use:     "firewall",
	Aliases: []string{"fw"},
	Short:   "Manage machine firewall rules (remote, via REST API)",
}

var machineFwListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name>",
	Short: "List firewall rules",
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
			Rules []map[string]interface{} `json:"rules"`
		}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/firewall", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Rules, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "PORT", Field: "port"},
			{Header: "PROTOCOL", Field: "protocol"},
			{Header: "SOURCE", Field: "source"},
			{Header: "DESCRIPTION", Field: "description"},
		})
	},
}

var machineFwAddCmd = &cobra.Command{
	Use:   "add <machine-id-or-name>",
	Short: "Add a firewall rule",
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

		port, _ := cmd.Flags().GetInt("port")
		protocol, _ := cmd.Flags().GetString("protocol")
		source, _ := cmd.Flags().GetString("source")
		description, _ := cmd.Flags().GetString("description")

		if port == 0 {
			return fmt.Errorf("--port is required")
		}

		body := map[string]interface{}{
			"port":     port,
			"protocol": protocol,
		}
		if source != "" {
			body["source"] = source
		}
		if description != "" {
			body["description"] = description
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/firewall", body, &resp); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Firewall rule added for port %d/%s.\n", port, protocol)
		return nil
	},
}

var machineFwRemoveCmd = &cobra.Command{
	Use:   "remove <machine-id-or-name> <rule-id>",
	Short: "Remove a firewall rule",
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

		if err := client.Delete(cmd.Context(), "/api/machines/"+id+"/firewall/"+args[1]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Firewall rule removed.")
		return nil
	},
}

func init() {
	machineFwAddCmd.Flags().Int("port", 0, "Port number")
	machineFwAddCmd.Flags().String("protocol", "tcp", "Protocol (tcp, udp)")
	machineFwAddCmd.Flags().String("source", "", "Source CIDR (default: 0.0.0.0/0)")
	machineFwAddCmd.Flags().String("description", "", "Rule description")

	machineFwCmd.AddCommand(machineFwListCmd)
	machineFwCmd.AddCommand(machineFwAddCmd)
	machineFwCmd.AddCommand(machineFwRemoveCmd)
}
