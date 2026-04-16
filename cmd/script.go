package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var scriptCmd = &cobra.Command{
	Use:   "script",
	Short: "Manage scripts",
}

var scriptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scripts",
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

		q := buildListQuery(cmd, url.Values{"projectId": {projectID}})

		var resp struct {
			Scripts []map[string]interface{} `json:"scripts"`
		}
		if err := client.Get(cmd.Context(), "/api/scripts", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Scripts, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "LANGUAGE", Field: "language"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var scriptCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a script",
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
		if cmd.Flags().Changed("language") {
			v, _ := cmd.Flags().GetString("language")
			overrides["language"] = v
		}
		if cmd.Flags().Changed("content") {
			v, _ := cmd.Flags().GetString("content")
			overrides["content"] = v
		}
		if cmd.Flags().Changed("content-file") {
			path, _ := cmd.Flags().GetString("content-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			overrides["content"] = content
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/scripts", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var scriptGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get script details",
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

		id, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/scripts/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "LANGUAGE", Field: "language"},
			{Header: "CONTENT", Field: "content", Width: 120},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var scriptEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Edit a script",
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

		id, err := resolveResource(cmd, f, "script", args[0], projectID)
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
		if cmd.Flags().Changed("content") {
			v, _ := cmd.Flags().GetString("content")
			overrides["content"] = v
		}
		if cmd.Flags().Changed("content-file") {
			path, _ := cmd.Flags().GetString("content-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			overrides["content"] = content
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/scripts/"+id, body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var scriptDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete a script",
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

		id, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete script %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/scripts/"+id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Script %s deleted.\n", args[0])
		return nil
	},
}

var scriptRunCmd = &cobra.Command{
	Use:   "run <id-or-name>",
	Short: "Execute a script",
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

		id, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("machine") {
			v, _ := cmd.Flags().GetString("machine")
			machineID, err := resolveMachine(cmd, f, v)
			if err != nil {
				return err
			}
			body["machineId"] = machineID
		}
		if cmd.Flags().Changed("timeout") {
			v, _ := cmd.Flags().GetInt("timeout")
			body["timeoutSeconds"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/scripts/"+id+"/execute", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "EXECUTION ID", Field: "id"},
			{Header: "STATUS", Field: "status"},
			{Header: "OUTPUT", Field: "output", Width: 120},
		})
	},
}

var scriptRunSequenceCmd = &cobra.Command{
	Use:   "run-sequence",
	Short: "Execute a sequence of scripts",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
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
		if cmd.Flags().Changed("machine") {
			v, _ := cmd.Flags().GetString("machine")
			machineID, err := resolveMachine(cmd, f, v)
			if err != nil {
				return err
			}
			body["machineId"] = machineID
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/scripts/execute-sequence", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "SEQUENCE ID", Field: "sequenceId"},
			{Header: "STATUS", Field: "status"},
		})
	},
}

var scriptPinCmd = &cobra.Command{
	Use:   "pin <script-id-or-name> <machine-id-or-name>",
	Short: "Pin a script to a machine",
	Args:  cobra.ExactArgs(2),
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

		scriptID, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}
		machineID, err := resolveMachine(cmd, f, args[1])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"machineId": machineID,
			"action":    "pin",
		}
		if err := client.Post(cmd.Context(), "/api/scripts/"+scriptID+"/pin", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Script pinned to machine.")
		return nil
	},
}

var scriptUnpinCmd = &cobra.Command{
	Use:   "unpin <script-id-or-name> <machine-id-or-name>",
	Short: "Unpin a script from a machine",
	Args:  cobra.ExactArgs(2),
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

		scriptID, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}
		machineID, err := resolveMachine(cmd, f, args[1])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"machineId": machineID,
			"action":    "unpin",
		}
		if err := client.Post(cmd.Context(), "/api/scripts/"+scriptID+"/pin", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Script unpinned from machine.")
		return nil
	},
}

var scriptRunsCmd = &cobra.Command{
	Use:   "runs <script-id-or-name>",
	Short: "List script execution runs",
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

		id, err := resolveResource(cmd, f, "script", args[0], projectID)
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, url.Values{"scriptId": {id}})

		var resp struct {
			Runs []map[string]interface{} `json:"runs"`
		}
		if err := client.Get(cmd.Context(), "/api/scripts/"+id+"/runs", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Runs, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "STATUS", Field: "status"},
			{Header: "STARTED", Field: "startedAt"},
			{Header: "FINISHED", Field: "finishedAt"},
			{Header: "EXIT CODE", Field: "exitCode"},
		})
	},
}

var scriptRunOutputCmd = &cobra.Command{
	Use:   "run-output <execution-id>",
	Short: "Get output of a script execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/scripts/runs/"+args[0], nil, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "STATUS", Field: "status"},
			{Header: "OUTPUT", Field: "output"},
			{Header: "EXIT CODE", Field: "exitCode"},
		})
	},
}

var scriptInterruptCmd = &cobra.Command{
	Use:   "interrupt <execution-id>",
	Short: "Interrupt a running script execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{"action": "interrupt"}
		if err := client.Post(cmd.Context(), "/api/scripts/runs/"+args[0]+"/interrupt", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Interrupt signal sent.")
		return nil
	},
}

func init() {
	cmdutil.AddListFlags(scriptListCmd)

	scriptCreateCmd.Flags().String("name", "", "Script name")
	scriptCreateCmd.Flags().String("language", "", "Script language (bash, python, etc.)")
	scriptCreateCmd.Flags().String("content", "", "Script content")
	scriptCreateCmd.Flags().String("content-file", "", "Path to script file")
	cmdutil.AddJSONInput(scriptCreateCmd)

	scriptEditCmd.Flags().String("name", "", "Script name")
	scriptEditCmd.Flags().String("content", "", "Script content")
	scriptEditCmd.Flags().String("content-file", "", "Path to script file")
	cmdutil.AddJSONInput(scriptEditCmd)

	scriptRunCmd.Flags().String("machine", "", "Machine to run on")
	scriptRunCmd.Flags().Int("timeout", 0, "Timeout in seconds")

	scriptRunSequenceCmd.Flags().String("machine", "", "Machine to run on")
	cmdutil.AddJSONInput(scriptRunSequenceCmd)

	cmdutil.AddListFlags(scriptRunsCmd)

	scriptCmd.AddCommand(scriptListCmd)
	scriptCmd.AddCommand(scriptCreateCmd)
	scriptCmd.AddCommand(scriptGetCmd)
	scriptCmd.AddCommand(scriptEditCmd)
	scriptCmd.AddCommand(scriptDeleteCmd)
	scriptCmd.AddCommand(scriptRunCmd)
	scriptCmd.AddCommand(scriptRunSequenceCmd)
	scriptCmd.AddCommand(scriptPinCmd)
	scriptCmd.AddCommand(scriptUnpinCmd)
	scriptCmd.AddCommand(scriptRunsCmd)
	scriptCmd.AddCommand(scriptRunOutputCmd)
	scriptCmd.AddCommand(scriptInterruptCmd)
}
