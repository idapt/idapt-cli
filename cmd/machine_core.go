package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/idapt/idapt-cli/internal/resolve"
	"github.com/spf13/cobra"
)

func resolveMachine(cmd *cobra.Command, f *cmdutil.Factory, nameOrID string) (string, error) {
	if resolve.IsUUID(nameOrID) {
		return nameOrID, nil
	}
	projectID, err := resolveProjectFlag(cmd, f)
	if err != nil {
		return "", err
	}
	return resolveResource(cmd, f, "machine", nameOrID, projectID)
}

var machineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{}
		if cmd.Flags().Changed("project") || globalFlags.Project != "" {
			projectID, err := resolveProjectFlag(cmd, f)
			if err != nil {
				return err
			}
			q.Set("projectId", projectID)
		}
		q = buildListQuery(cmd, q)

		var resp struct {
			Machines []map[string]interface{} `json:"machines"`
		}
		if err := client.Get(cmd.Context(), "/api/machines", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Machines, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "STATE", Field: "state"},
			{Header: "TYPE", Field: "instanceType"},
			{Header: "IP", Field: "publicIp"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var machineCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a machine",
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

		body := map[string]interface{}{"projectId": projectID}
		if cmd.Flags().Changed("json") {
			raw, _ := cmd.Flags().GetString("json")
			parsed, err := input.ParseJSONFlag(raw, f.In)
			if err != nil {
				return err
			}
			body = input.MergeFlags(parsed, map[string]interface{}{"projectId": projectID})
		}

		overrides := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			overrides["name"] = v
		}
		if cmd.Flags().Changed("instance-type") {
			v, _ := cmd.Flags().GetString("instance-type")
			overrides["instanceType"] = v
		}
		if cmd.Flags().Changed("storage") {
			v, _ := cmd.Flags().GetInt("storage")
			overrides["rootVolumeSizeGb"] = v
		}
		if cmd.Flags().Changed("region") {
			v, _ := cmd.Flags().GetString("region")
			overrides["region"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "STATE", Field: "state"},
		})
	},
}

var machineGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get machine details",
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

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/machines/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "STATE", Field: "state"},
			{Header: "INSTANCE TYPE", Field: "instanceType"},
			{Header: "STORAGE GB", Field: "rootVolumeSizeGb"},
			{Header: "PUBLIC IP", Field: "publicIp"},
			{Header: "REGION", Field: "region"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var machineEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Edit a machine",
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

		body := map[string]interface{}{}
		if cmd.Flags().Changed("json") {
			raw, _ := cmd.Flags().GetString("json")
			parsed, err := input.ParseJSONFlag(raw, f.In)
			if err != nil {
				return err
			}
			body = parsed
		}

		overrides := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			overrides["name"] = v
		}
		if cmd.Flags().Changed("instance-type") {
			v, _ := cmd.Flags().GetString("instance-type")
			overrides["instanceType"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/machines/"+id, body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "STATE", Field: "state"},
		})
	},
}

var machineStartCmd = &cobra.Command{
	Use:   "start <id-or-name>",
	Short: "Start (wake) a hibernated machine",
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

		body := map[string]interface{}{"action": "wake"}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/action", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Machine %s starting.\n", args[0])
		return nil
	},
}

var machineStopCmd = &cobra.Command{
	Use:   "stop <id-or-name>",
	Short: "Stop (hibernate) a machine",
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

		body := map[string]interface{}{"action": "hibernate"}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/action", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Machine %s stopping.\n", args[0])
		return nil
	},
}

var machineTerminateCmd = &cobra.Command{
	Use:   "terminate <id-or-name>",
	Short: "Terminate a machine permanently",
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

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Terminate machine %s? This is irreversible.", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		body := map[string]interface{}{"action": "terminate"}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/action", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Machine %s terminating.\n", args[0])
		return nil
	},
}

var machineActivityCmd = &cobra.Command{
	Use:   "activity <id-or-name>",
	Short: "Show recent machine activity",
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
			Activity []map[string]interface{} `json:"activity"`
		}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/activity", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Activity, []output.Column{
			{Header: "TIME", Field: "timestamp"},
			{Header: "TYPE", Field: "type"},
			{Header: "DETAIL", Field: "detail", Width: 60},
		})
	},
}

func init() {
	cmdutil.AddListFlags(machineListCmd)

	machineCreateCmd.Flags().String("name", "", "Machine name (subdomain)")
	machineCreateCmd.Flags().String("instance-type", "", "Instance type (e.g. t3.micro)")
	machineCreateCmd.Flags().Int("storage", 0, "Root volume size in GB")
	machineCreateCmd.Flags().String("region", "", "AWS region")
	cmdutil.AddJSONInput(machineCreateCmd)

	machineEditCmd.Flags().String("name", "", "Machine name")
	machineEditCmd.Flags().String("instance-type", "", "Instance type")
	cmdutil.AddJSONInput(machineEditCmd)
}
