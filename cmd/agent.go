package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
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
			Agents []map[string]interface{} `json:"agents"`
		}
		if err := client.Get(cmd.Context(), "/api/agents", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Agents, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ICON", Field: "icon"},
			{Header: "MODEL", Field: "modelId"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent",
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
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		if cmd.Flags().Changed("system-prompt") {
			v, _ := cmd.Flags().GetString("system-prompt")
			overrides["systemPrompt"] = v
		}
		if cmd.Flags().Changed("system-prompt-file") {
			path, _ := cmd.Flags().GetString("system-prompt-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return fmt.Errorf("reading system prompt file: %w", err)
			}
			overrides["systemPrompt"] = content
		}
		if cmd.Flags().Changed("permission-preset") {
			v, _ := cmd.Flags().GetString("permission-preset")
			overrides["permissionPreset"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/agents", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ICON", Field: "icon"},
		})
	},
}

var agentGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get agent details",
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

		id, err := resolveResource(cmd, f, "agent", args[0], projectID)
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/agents/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ICON", Field: "icon"},
			{Header: "MODEL", Field: "modelId"},
			{Header: "SYSTEM PROMPT", Field: "systemPrompt", Width: 80},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var agentEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Edit an agent",
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

		id, err := resolveResource(cmd, f, "agent", args[0], projectID)
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
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		if cmd.Flags().Changed("system-prompt") {
			v, _ := cmd.Flags().GetString("system-prompt")
			overrides["systemPrompt"] = v
		}
		if cmd.Flags().Changed("system-prompt-file") {
			path, _ := cmd.Flags().GetString("system-prompt-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return fmt.Errorf("reading system prompt file: %w", err)
			}
			overrides["systemPrompt"] = content
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/agents/"+id, body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete an agent",
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

		id, err := resolveResource(cmd, f, "agent", args[0], projectID)
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete agent %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/agents/"+id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Agent %s deleted.\n", args[0])
		return nil
	},
}

func init() {
	cmdutil.AddListFlags(agentListCmd)

	agentCreateCmd.Flags().String("name", "", "Agent name")
	agentCreateCmd.Flags().String("icon", "", "Agent icon emoji")
	agentCreateCmd.Flags().String("system-prompt", "", "System prompt text")
	agentCreateCmd.Flags().String("system-prompt-file", "", "Path to system prompt file")
	agentCreateCmd.Flags().String("permission-preset", "", "Permission preset (full, read-only, custom)")
	cmdutil.AddJSONInput(agentCreateCmd)

	agentEditCmd.Flags().String("name", "", "Agent name")
	agentEditCmd.Flags().String("icon", "", "Agent icon emoji")
	agentEditCmd.Flags().String("system-prompt", "", "System prompt text")
	agentEditCmd.Flags().String("system-prompt-file", "", "Path to system prompt file")
	cmdutil.AddJSONInput(agentEditCmd)

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentGetCmd)
	agentCmd.AddCommand(agentEditCmd)
	agentCmd.AddCommand(agentDeleteCmd)
}
