package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Manage chats",
}

var chatListCmd = &cobra.Command{
	Use:   "list",
	Short: "List chats",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{}
		if cmd.Flags().Changed("agent") {
			agentID, _ := cmd.Flags().GetString("agent")
			q.Set("agentId", agentID)
		}
		if cmd.Flags().Changed("project") || globalFlags.Project != "" {
			projectID, err := resolveProjectFlag(cmd, f)
			if err != nil {
				return err
			}
			q.Set("projectId", projectID)
		}
		q = buildListQuery(cmd, q)

		var resp struct {
			Chats []map[string]interface{} `json:"chats"`
		}
		if err := client.Get(cmd.Context(), "/api/chat", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Chats, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title", Width: 50},
			{Header: "AGENT", Field: "agentId"},
			{Header: "MODEL", Field: "selectedContextModel"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var chatCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new chat",
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
		if cmd.Flags().Changed("agent") {
			v, _ := cmd.Flags().GetString("agent")
			overrides["agentId"] = v
		}
		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			overrides["selectedContextModel"] = v
		}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			overrides["title"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/chat", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
		})
	},
}

var chatGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get chat details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/chat/"+args[0], nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
			{Header: "AGENT", Field: "agentId"},
			{Header: "MODEL", Field: "selectedContextModel"},
			{Header: "STATUS", Field: "currentAgentRunId"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var chatEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a chat",
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
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			overrides["title"] = v
		}
		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			overrides["selectedContextModel"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/chat/"+args[0], body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
		})
	},
}

var chatDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a chat",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete chat %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/chat/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Chat %s deleted.\n", args[0])
		return nil
	},
}

var chatSendCmd = &cobra.Command{
	Use:   "send <chat-id> <message>",
	Short: "Send a message to a chat and stream the response",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		chatID := args[0]
		message := args[1]

		body := map[string]interface{}{
			"content": map[string]interface{}{
				"text": message,
			},
		}

		model, _ := cmd.Flags().GetString("model")
		if model != "" {
			body["selectedContextModel"] = model
		}

		var pending map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/chat/"+chatID+"/pending", body, &pending); err != nil {
			return err
		}

		noStream, _ := cmd.Flags().GetBool("no-stream")
		if noStream {
			fmt.Fprintln(cmd.OutOrStdout(), "Message sent. Use `idapt chat messages` to view responses.")
			return nil
		}

		reader, err := client.StreamSSEGet(cmd.Context(), "/api/chat/"+chatID+"/stream")
		if err != nil {
			return err
		}
		defer reader.Close()

		for {
			event, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			switch event.Event {
			case "text-delta":
				var delta struct {
					Text string `json:"text"`
				}
				if json.Unmarshal([]byte(event.Data), &delta) == nil {
					fmt.Fprint(cmd.OutOrStdout(), delta.Text)
				}
			case "done", "error":
				fmt.Fprintln(cmd.OutOrStdout())
				if event.Event == "error" {
					return fmt.Errorf("stream error: %s", event.Data)
				}
				return nil
			}
		}

		fmt.Fprintln(cmd.OutOrStdout())
		return nil
	},
}

var chatMessagesCmd = &cobra.Command{
	Use:   "messages <chat-id>",
	Short: "List messages in a chat",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, nil)

		var resp struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := client.Get(cmd.Context(), "/api/chat/"+args[0]+"/messages", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Messages, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "type"},
			{Header: "TEXT", Field: "userText", Width: 80},
			{Header: "ASSISTANT", Field: "assistantText", Width: 80},
		})
	},
}

var chatExportCmd = &cobra.Command{
	Use:   "export <chat-id>",
	Short: "Export a chat as markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		result, err := client.Download(cmd.Context(), "/api/chat/"+args[0]+"/export")
		if err != nil {
			return err
		}
		defer result.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), result.Body)
		return err
	},
}

var chatStopCmd = &cobra.Command{
	Use:   "stop <chat-id>",
	Short: "Stop an active chat generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		now := time.Now().UTC().Format(time.RFC3339)
		body := map[string]interface{}{
			"stopRequestedAt": now,
		}

		if err := client.Patch(cmd.Context(), "/api/chat/"+args[0], body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Stop requested.")
		return nil
	},
}

func init() {
	chatListCmd.Flags().String("agent", "", "Filter by agent ID")
	cmdutil.AddListFlags(chatListCmd)

	chatCreateCmd.Flags().String("agent", "", "Agent ID")
	chatCreateCmd.Flags().String("model", "", "Model ID")
	chatCreateCmd.Flags().String("title", "", "Chat title")
	cmdutil.AddJSONInput(chatCreateCmd)

	chatEditCmd.Flags().String("title", "", "Chat title")
	chatEditCmd.Flags().String("model", "", "Model ID")
	cmdutil.AddJSONInput(chatEditCmd)

	chatSendCmd.Flags().String("model", "", "Override model for this message")
	chatSendCmd.Flags().Bool("no-stream", false, "Don't stream the response")

	cmdutil.AddListFlags(chatMessagesCmd)

	chatCmd.AddCommand(chatListCmd)
	chatCmd.AddCommand(chatCreateCmd)
	chatCmd.AddCommand(chatGetCmd)
	chatCmd.AddCommand(chatEditCmd)
	chatCmd.AddCommand(chatDeleteCmd)
	chatCmd.AddCommand(chatSendCmd)
	chatCmd.AddCommand(chatMessagesCmd)
	chatCmd.AddCommand(chatExportCmd)
	chatCmd.AddCommand(chatStopCmd)
}
