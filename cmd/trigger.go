package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)
var triggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Manage triggers (cron schedules + webhook endpoints)",
	Long: `Triggers fire an action (agent run or code execution) on a schedule
or when their webhook endpoint receives an authenticated POST. Every trigger
is project-scoped.`,
}

var triggerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List triggers for the current user",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, url.Values{})

		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/triggers", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "trigger_type"},
			{Header: "ACTION", Field: "action_type"},
			{Header: "ENABLED", Field: "enabled"},
			{Header: "NEXT RUN", Field: "next_run_at"},
			{Header: "LAST FIRED", Field: "last_fired_at"},
		})
	},
}

var triggerGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get trigger details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/triggers/"+args[0], nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "trigger_type"},
			{Header: "ACTION", Field: "action_type"},
			{Header: "ENABLED", Field: "enabled"},
			{Header: "DESCRIPTION", Field: "description"},
			{Header: "NEXT RUN", Field: "next_run_at"},
			{Header: "LAST FIRED", Field: "last_fired_at"},
			{Header: "CREATED", Field: "created_at"},
		})
	},
}

var triggerCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a trigger",
	Long: `Create a cron or webhook trigger. Build the request body with --json
or via individual flags. When triggerType=webhook, the response includes a
one-time "secret" field — store it immediately, it is never shown again.`,
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
		addStringFlag(cmd, overrides, "name", "name")
		addStringFlag(cmd, overrides, "description", "description")
		addStringFlag(cmd, overrides, "trigger-type", "trigger_type")
		addStringFlag(cmd, overrides, "action-type", "action_type")
		addStringFlag(cmd, overrides, "cron-expression", "cron_expression")
		addStringFlag(cmd, overrides, "cron-timezone", "cron_timezone")
		addStringFlag(cmd, overrides, "agent-id", "agent_id")
		addStringFlag(cmd, overrides, "prompt-template", "prompt_template")
		addStringFlag(cmd, overrides, "model", "model")
		addStringFlag(cmd, overrides, "file-id", "file_id")
		addStringFlag(cmd, overrides, "runtime", "runtime")
		if cmd.Flags().Changed("timeout-seconds") {
			v, _ := cmd.Flags().GetInt("timeout-seconds")
			overrides["timeout_seconds"] = v
		}
		if cmd.Flags().Changed("reasoning-level") {
			v, _ := cmd.Flags().GetInt("reasoning-level")
			overrides["reasoning_level"] = v
		}
		if cmd.Flags().Changed("disabled") {
			v, _ := cmd.Flags().GetBool("disabled")
			overrides["enabled"] = !v
		}
		body = input.MergeFlags(body, overrides)

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}

		httpResp, err := client.Do(
			cmd.Context(),
			"POST",
			"/api/v1/triggers",
			bytes.NewReader(bodyBytes),
			api.WithHeader("Content-Type", "application/json"),
			api.WithQuery(url.Values{"project_id": {projectID}}),
		)
		if err != nil {
			return err
		}
		defer httpResp.Body.Close()

		var resp struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		formatter := f.Formatter()
		cols := []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "trigger_type"},
			{Header: "ACTION", Field: "action_type"},
			{Header: "ENABLED", Field: "enabled"},
		}
		if _, ok := resp.Data["secret"]; ok {
			cols = append(cols, output.Column{Header: "SECRET (SHOWN ONCE)", Field: "secret"})
		}
		return formatter.WriteItem(resp.Data, cols)
	},
}

var triggerEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Update a trigger",
	Args:  cobra.ExactArgs(1),
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

		overrides := map[string]interface{}{}
		addStringFlag(cmd, overrides, "name", "name")
		addStringFlag(cmd, overrides, "description", "description")
		addStringFlag(cmd, overrides, "cron-expression", "cron_expression")
		addStringFlag(cmd, overrides, "cron-timezone", "cron_timezone")
		addStringFlag(cmd, overrides, "agent-id", "agent_id")
		addStringFlag(cmd, overrides, "prompt-template", "prompt_template")
		addStringFlag(cmd, overrides, "model", "model")
		addStringFlag(cmd, overrides, "file-id", "file_id")
		addStringFlag(cmd, overrides, "runtime", "runtime")
		if cmd.Flags().Changed("timeout-seconds") {
			v, _ := cmd.Flags().GetInt("timeout-seconds")
			overrides["timeout_seconds"] = v
		}
		if cmd.Flags().Changed("reasoning-level") {
			v, _ := cmd.Flags().GetInt("reasoning-level")
			overrides["reasoning_level"] = v
		}
		if cmd.Flags().Changed("enabled") {
			v, _ := cmd.Flags().GetBool("enabled")
			overrides["enabled"] = v
		}
		body = input.MergeFlags(body, overrides)

		if len(body) == 0 {
			return fmt.Errorf("at least one field to update is required (use --json or a field flag)")
		}

		var resp struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := client.Patch(cmd.Context(), "/api/v1/triggers/"+args[0], body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ENABLED", Field: "enabled"},
			{Header: "NEXT RUN", Field: "next_run_at"},
		})
	},
}

var triggerDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a trigger and its run history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete trigger %s and its run history?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/v1/triggers/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Trigger %s deleted.\n", args[0])
		return nil
	},
}

var triggerRotateSecretCmd = &cobra.Command{
	Use:   "rotate-secret <id>",
	Short: "Rotate the webhook secret for a trigger (old one becomes invalid)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Rotate webhook secret for %s? Current secret will stop working.", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		var resp struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := client.Post(cmd.Context(), "/api/v1/triggers/"+args[0]+"/rotate-secret", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "NEW SECRET (SHOWN ONCE)", Field: "secret"},
		})
	},
}

var triggerRunsCmd = &cobra.Command{
	Use:   "runs <id>",
	Short: "List recent fire attempts (successes and failures)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, url.Values{})

		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/triggers/"+args[0]+"/runs", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "FIRED AT", Field: "fired_at"},
			{Header: "SUCCESS", Field: "success"},
			{Header: "CHAT", Field: "chat_id"},
			{Header: "EXEC RUN", Field: "execution_run_id"},
			{Header: "ERROR", Field: "error"},
		})
	},
}

var triggerFireCmd = &cobra.Command{
	Use:   "fire <id>",
	Short: "Fire a webhook trigger (requires the trigger's secret)",
	Long: `Fires a trigger via its webhook endpoint. This does NOT use your API
key — the bearer is the trigger's one-time secret, captured when it was
created. If you lost it, run 'idapt trigger rotate-secret <id>' first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		secret, _ := cmd.Flags().GetString("secret")
		if secret == "" {
			return fmt.Errorf("--secret is required (the trigger's webhook bearer)")
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

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}

		httpResp, err := client.Do(
			cmd.Context(),
			"POST",
			"/api/v1/triggers/"+args[0]+"/fire",
			bytes.NewReader(bodyBytes),
			api.WithHeader("Content-Type", "application/json"),
			api.WithHeader("Authorization", "Bearer "+secret),
		)
		if err != nil {
			return err
		}
		defer httpResp.Body.Close()

		var resp struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data, []output.Column{
			{Header: "TRIGGER ID", Field: "trigger_id"},
			{Header: "STATUS", Field: "status"},
		})
	},
}

func addStringFlag(cmd *cobra.Command, dst map[string]interface{}, flag, key string) {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		dst[key] = v
	}
}

func init() {
	cmdutil.AddListFlags(triggerListCmd)
	cmdutil.AddListFlags(triggerRunsCmd)

	triggerCreateCmd.Flags().String("name", "", "Trigger name")
	triggerCreateCmd.Flags().String("description", "", "Trigger description")
	triggerCreateCmd.Flags().String("trigger-type", "", "Trigger source: cron or webhook")
	triggerCreateCmd.Flags().String("action-type", "", "Action type: agent-run or code-execution")
	triggerCreateCmd.Flags().String("cron-expression", "", "5-field cron expression (for cron triggers)")
	triggerCreateCmd.Flags().String("cron-timezone", "", "IANA timezone (default UTC)")
	triggerCreateCmd.Flags().String("agent-id", "", "Agent UUID (for agent-run action)")
	triggerCreateCmd.Flags().String("prompt-template", "", "Prompt template (for agent-run action)")
	triggerCreateCmd.Flags().String("model", "", "Override model id (agent-run)")
	triggerCreateCmd.Flags().Int("reasoning-level", 0, "Reasoning effort 0-100 (agent-run)")
	triggerCreateCmd.Flags().String("file-id", "", "Code file UUID (code-execution)")
	triggerCreateCmd.Flags().String("runtime", "", "Runtime override (code-execution)")
	triggerCreateCmd.Flags().Int("timeout-seconds", 0, "Execution timeout 1-300s (code-execution)")
	triggerCreateCmd.Flags().Bool("disabled", false, "Create in disabled state")
	cmdutil.AddJSONInput(triggerCreateCmd)

	triggerEditCmd.Flags().String("name", "", "Trigger name")
	triggerEditCmd.Flags().String("description", "", "Trigger description")
	triggerEditCmd.Flags().String("cron-expression", "", "5-field cron expression")
	triggerEditCmd.Flags().String("cron-timezone", "", "IANA timezone")
	triggerEditCmd.Flags().String("agent-id", "", "Agent UUID")
	triggerEditCmd.Flags().String("prompt-template", "", "Prompt template")
	triggerEditCmd.Flags().String("model", "", "Model id override")
	triggerEditCmd.Flags().Int("reasoning-level", 0, "Reasoning effort 0-100")
	triggerEditCmd.Flags().String("file-id", "", "Code file UUID")
	triggerEditCmd.Flags().String("runtime", "", "Runtime override")
	triggerEditCmd.Flags().Int("timeout-seconds", 0, "Execution timeout 1-300s")
	triggerEditCmd.Flags().Bool("enabled", true, "Enable/disable the trigger")
	cmdutil.AddJSONInput(triggerEditCmd)

	triggerFireCmd.Flags().String("secret", "", "Webhook secret (the one captured at create/rotate time)")
	cmdutil.AddJSONInput(triggerFireCmd)

	triggerCmd.AddCommand(triggerListCmd)
	triggerCmd.AddCommand(triggerGetCmd)
	triggerCmd.AddCommand(triggerCreateCmd)
	triggerCmd.AddCommand(triggerEditCmd)
	triggerCmd.AddCommand(triggerDeleteCmd)
	triggerCmd.AddCommand(triggerRotateSecretCmd)
	triggerCmd.AddCommand(triggerRunsCmd)
	triggerCmd.AddCommand(triggerFireCmd)
}
