package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)
var notificationCmd = &cobra.Command{
	Use:     "notification",
	Aliases: []string{"notif"},
	Short:   "Manage notifications (inbox + project broadcasts)",
	Long: `Manage notifications via the public v1 API. List your inbox, send
notifications to project members, manage channel preferences, and configure
quiet hours.`,
}
var notificationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notifications in the caller's inbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, url.Values{})
		if cmd.Flags().Changed("unread") {
			q.Set("unread_only", "true")
		}
		if cmd.Flags().Changed("archived") {
			q.Set("archived_only", "true")
		}
		if cmd.Flags().Changed("type") {
			v, _ := cmd.Flags().GetString("type")
			q.Set("type", v)
		}
		if cmd.Flags().Changed("project-id") {
			v, _ := cmd.Flags().GetString("project-id")
			q.Set("project_id", v)
		}
		if cmd.Flags().Changed("search") {
			v, _ := cmd.Flags().GetString("search")
			q.Set("search", v)
		}

		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/notifications", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "type"},
			{Header: "TITLE", Field: "title", Width: 40},
			{Header: "MESSAGE", Field: "message", Width: 50},
			{Header: "READ", Field: "is_read"},
			{Header: "CREATED", Field: "created_at"},
		})
	},
}
var notificationGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a single notification",
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
		if err := client.Get(cmd.Context(), "/api/v1/notifications/"+args[0], nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "type"},
			{Header: "SUBTYPE", Field: "subtype"},
			{Header: "TITLE", Field: "title"},
			{Header: "MESSAGE", Field: "message", Width: 80},
			{Header: "PROJECT", Field: "project_id"},
			{Header: "SENDER", Field: "sender_kind"},
			{Header: "READ", Field: "is_read"},
			{Header: "READ AT", Field: "read_at"},
			{Header: "ARCHIVED AT", Field: "archived_at"},
			{Header: "CREATED", Field: "created_at"},
		})
	},
}
var notificationReadCmd = &cobra.Command{
	Use:   "read [id]",
	Short: "Mark notification(s) as read",
	Long:  "Mark one notification (by id) as read, or all if no id is provided.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if len(args) > 0 {
			body := map[string]interface{}{"read": true}
			if err := client.Patch(cmd.Context(), "/api/v1/notifications/"+args[0], body, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Notification marked as read.")
			return nil
		}
		if err := client.Post(cmd.Context(), "/api/v1/notifications/read-all", nil, nil); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "All notifications marked as read.")
		return nil
	},
}
var notificationArchiveCmd = &cobra.Command{
	Use:   "archive <id>",
	Short: "Archive a notification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}
		body := map[string]interface{}{"archived": true}
		if err := client.Patch(cmd.Context(), "/api/v1/notifications/"+args[0], body, nil); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Notification archived.")
		return nil
	},
}

var notificationUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <id>",
	Short: "Move an archived notification back to the inbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}
		body := map[string]interface{}{"unarchive": true}
		if err := client.Patch(cmd.Context(), "/api/v1/notifications/"+args[0], body, nil); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Notification unarchived.")
		return nil
	},
}

var notificationDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a notification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}
		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete notification %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}
		if err := client.Delete(cmd.Context(), "/api/v1/notifications/"+args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Notification deleted.")
		return nil
	},
}
var notificationSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a notification to project members",
	Long: `Send a notification to one or more members of a project.

Choose your audience with --target (named audience) or --to (explicit
profile ids). Caller must be a member of the project. Channel preferences
are intersected with each recipient's per-type settings — user opt-outs
always win.`,
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

		overrides := map[string]interface{}{
			"project_id": projectID,
		}
		addStringFlag(cmd, overrides, "title", "title")
		addStringFlag(cmd, overrides, "message", "message")
		addStringFlag(cmd, overrides, "target", "target")
		addStringFlag(cmd, overrides, "urgency", "urgency")
		addStringFlag(cmd, overrides, "dedup-key", "dedup_key")
		if cmd.Flags().Changed("to") {
			recipients, _ := cmd.Flags().GetStringSlice("to")
			overrides["recipient_ids"] = recipients
		}
		if cmd.Flags().Changed("channels") {
			channels, _ := cmd.Flags().GetStringSlice("channels")
			overrides["channels"] = channels
		}
		if cmd.Flags().Changed("deep-link-kind") {
			kind, _ := cmd.Flags().GetString("deep-link-kind")
			dl := map[string]interface{}{"kind": kind}
			if v, _ := cmd.Flags().GetString("deep-link-chat-id"); v != "" {
				dl["chat_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-message-id"); v != "" {
				dl["message_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-file-id"); v != "" {
				dl["file_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-project-id"); v != "" {
				dl["project_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-agent-id"); v != "" {
				dl["agent_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-machine-id"); v != "" {
				dl["machine_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-ticket-id"); v != "" {
				dl["ticket_id"] = v
			}
			if v, _ := cmd.Flags().GetString("deep-link-section"); v != "" {
				dl["section"] = v
			}
			overrides["deep_link"] = dl
		}
		body = input.MergeFlags(body, overrides)

		title, _ := body["title"].(string)
		message, _ := body["message"].(string)
		if strings.TrimSpace(title) == "" || strings.TrimSpace(message) == "" {
			return fmt.Errorf("--title and --message are required")
		}
		if _, hasTarget := body["target"]; !hasTarget {
			if _, hasRecipients := body["recipient_ids"]; !hasRecipients {
				return fmt.Errorf("either --target or --to is required")
			}
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}

		httpResp, err := client.Do(
			cmd.Context(),
			"POST",
			"/api/v1/notifications",
			bytes.NewReader(bodyBytes),
			api.WithHeader("Content-Type", "application/json"),
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
			{Header: "EVENT ID", Field: "event_id"},
			{Header: "RECIPIENTS", Field: "recipient_count"},
			{Header: "DEDUPED", Field: "deduped"},
		})
	},
}
var notificationPreferencesCmd = &cobra.Command{
	Use:     "preferences",
	Aliases: []string{"prefs"},
	Short:   "View or update notification preferences",
}

var notificationPreferencesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the per-(type, channel) preference matrix",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}
		var resp struct {
			Data struct {
				Preferences []map[string]interface{} `json:"preferences"`
			} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/notifications/preferences", nil, &resp); err != nil {
			return err
		}
		formatter := f.Formatter()
		return formatter.WriteList(resp.Data.Preferences, []output.Column{
			{Header: "TYPE", Field: "type"},
			{Header: "SUBTYPE", Field: "subtype"},
			{Header: "CHANNEL", Field: "channel"},
			{Header: "ENABLED", Field: "enabled"},
			{Header: "DEFAULT", Field: "is_default"},
		})
	},
}

var notificationPreferencesSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update one or more preferences",
	Long: `Update preferences. Provide at least one --type/--channel/--enabled
trio, or pass a JSON body via --json containing {"updates":[...]}.`,
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

		if cmd.Flags().Changed("type") {
			t, _ := cmd.Flags().GetString("type")
			ch, _ := cmd.Flags().GetString("channel")
			if ch == "" {
				return fmt.Errorf("--channel is required when --type is set")
			}
			if !cmd.Flags().Changed("enabled") {
				return fmt.Errorf("--enabled is required when --type is set")
			}
			enabled, _ := cmd.Flags().GetBool("enabled")
			update := map[string]interface{}{
				"type":    t,
				"channel": ch,
				"enabled": enabled,
			}
			if cmd.Flags().Changed("subtype") {
				st, _ := cmd.Flags().GetString("subtype")
				update["subtype"] = st
			}
			body["updates"] = []map[string]interface{}{update}
		}

		if _, ok := body["updates"]; !ok {
			return fmt.Errorf("provide --json with updates[] or --type/--channel/--enabled")
		}

		var resp struct {
			Data struct {
				Preferences []map[string]interface{} `json:"preferences"`
			} `json:"data"`
		}
		if err := client.Patch(cmd.Context(), "/api/v1/notifications/preferences", body, &resp); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Preferences updated.")
		return nil
	},
}
var notificationConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update notification config (quiet hours, toasts, sound)",
}

var notificationConfigGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show notification config",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}
		var resp struct {
			Data struct {
				Config map[string]interface{} `json:"config"`
			} `json:"data"`
		}
		if err := client.Get(cmd.Context(), "/api/v1/notifications/config", nil, &resp); err != nil {
			return err
		}
		formatter := f.Formatter()
		return formatter.WriteItem(resp.Data.Config, []output.Column{
			{Header: "TOASTS", Field: "toasts_enabled"},
			{Header: "SOUND", Field: "sound_enabled"},
			{Header: "QUIET HOURS", Field: "quiet_hours_enabled"},
			{Header: "QUIET START", Field: "quiet_hours_start"},
			{Header: "QUIET END", Field: "quiet_hours_end"},
			{Header: "TIMEZONE", Field: "quiet_hours_timezone"},
			{Header: "DIGEST", Field: "digest_frequency"},
		})
	},
}

var notificationConfigSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update notification config",
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
		if cmd.Flags().Changed("toasts") {
			v, _ := cmd.Flags().GetBool("toasts")
			body["toasts_enabled"] = v
		}
		if cmd.Flags().Changed("sound") {
			v, _ := cmd.Flags().GetBool("sound")
			body["sound_enabled"] = v
		}
		if cmd.Flags().Changed("quiet-hours") {
			v, _ := cmd.Flags().GetBool("quiet-hours")
			body["quiet_hours_enabled"] = v
		}
		if cmd.Flags().Changed("quiet-start") {
			v, _ := cmd.Flags().GetString("quiet-start")
			body["quiet_hours_start"] = v
		}
		if cmd.Flags().Changed("quiet-end") {
			v, _ := cmd.Flags().GetString("quiet-end")
			body["quiet_hours_end"] = v
		}
		if cmd.Flags().Changed("timezone") {
			v, _ := cmd.Flags().GetString("timezone")
			body["quiet_hours_timezone"] = v
		}
		if cmd.Flags().Changed("digest") {
			v, _ := cmd.Flags().GetString("digest")
			body["digest_frequency"] = v
		}

		if len(body) == 0 {
			return fmt.Errorf("at least one field is required (--toasts, --sound, --quiet-hours, --quiet-start, --quiet-end, --timezone, --digest, or --json)")
		}

		if err := client.Patch(cmd.Context(), "/api/v1/notifications/config", body, nil); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated.")
		return nil
	},
}
func init() {
	cmdutil.AddListFlags(notificationListCmd)
	notificationListCmd.Flags().Bool("unread", false, "Only show unread notifications")
	notificationListCmd.Flags().Bool("archived", false, "Only show archived notifications")
	notificationListCmd.Flags().String("type", "", "Filter by notification type")
	notificationListCmd.Flags().String("project-id", "", "Filter by project ID")
	notificationListCmd.Flags().String("search", "", "Case-insensitive search over title + message")

	notificationSendCmd.Flags().String("title", "", "Notification title (1-120 chars)")
	notificationSendCmd.Flags().String("message", "", "Notification body (1-500 chars)")
	notificationSendCmd.Flags().String("target", "", "Named audience: all_members | admins | owner")
	notificationSendCmd.Flags().StringSlice("to", nil, "Explicit recipient profile id(s) (alternative to --target)")
	notificationSendCmd.Flags().StringSlice("channels", nil, "Preferred channels (in_app,email,web_push)")
	notificationSendCmd.Flags().String("urgency", "", "low | normal | high")
	notificationSendCmd.Flags().String("dedup-key", "", "Per-recipient idempotency key (24h)")
	notificationSendCmd.Flags().String("deep-link-kind", "", "Deep-link target kind: chat | file | project | agent | machine | settings | billing | usage | support-ticket | hub | home")
	notificationSendCmd.Flags().String("deep-link-chat-id", "", "Required when --deep-link-kind=chat")
	notificationSendCmd.Flags().String("deep-link-message-id", "", "Optional message anchor for --deep-link-kind=chat")
	notificationSendCmd.Flags().String("deep-link-file-id", "", "Required when --deep-link-kind=file")
	notificationSendCmd.Flags().String("deep-link-project-id", "", "Required when --deep-link-kind=project")
	notificationSendCmd.Flags().String("deep-link-agent-id", "", "Required when --deep-link-kind=agent")
	notificationSendCmd.Flags().String("deep-link-machine-id", "", "Required when --deep-link-kind=machine")
	notificationSendCmd.Flags().String("deep-link-ticket-id", "", "Required when --deep-link-kind=support-ticket")
	notificationSendCmd.Flags().String("deep-link-section", "", "Optional section for settings/project/agent/machine/hub kinds")
	cmdutil.AddJSONInput(notificationSendCmd)

	notificationPreferencesSetCmd.Flags().String("type", "", "Notification type")
	notificationPreferencesSetCmd.Flags().String("subtype", "", "Notification subtype")
	notificationPreferencesSetCmd.Flags().String("channel", "", "in_app | email | web_push")
	notificationPreferencesSetCmd.Flags().Bool("enabled", true, "Enable or disable this channel for the type")
	cmdutil.AddJSONInput(notificationPreferencesSetCmd)

	notificationConfigSetCmd.Flags().Bool("toasts", true, "Enable in-app toasts")
	notificationConfigSetCmd.Flags().Bool("sound", true, "Enable notification sound")
	notificationConfigSetCmd.Flags().Bool("quiet-hours", false, "Enable quiet hours")
	notificationConfigSetCmd.Flags().String("quiet-start", "", "Quiet hours start HH:MM")
	notificationConfigSetCmd.Flags().String("quiet-end", "", "Quiet hours end HH:MM")
	notificationConfigSetCmd.Flags().String("timezone", "", "IANA timezone for quiet hours (e.g. Europe/Paris)")
	notificationConfigSetCmd.Flags().String("digest", "", "immediate | hourly | daily")
	cmdutil.AddJSONInput(notificationConfigSetCmd)

	notificationPreferencesCmd.AddCommand(notificationPreferencesGetCmd)
	notificationPreferencesCmd.AddCommand(notificationPreferencesSetCmd)
	notificationConfigCmd.AddCommand(notificationConfigGetCmd)
	notificationConfigCmd.AddCommand(notificationConfigSetCmd)

	notificationCmd.AddCommand(notificationListCmd)
	notificationCmd.AddCommand(notificationGetCmd)
	notificationCmd.AddCommand(notificationReadCmd)
	notificationCmd.AddCommand(notificationArchiveCmd)
	notificationCmd.AddCommand(notificationUnarchiveCmd)
	notificationCmd.AddCommand(notificationDeleteCmd)
	notificationCmd.AddCommand(notificationSendCmd)
	notificationCmd.AddCommand(notificationPreferencesCmd)
	notificationCmd.AddCommand(notificationConfigCmd)
}
