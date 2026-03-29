package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
}

var taskListCmd = &cobra.Command{
	Use:   "list <board-id>",
	Short: "List tasks on a board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, nil)
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			q.Set("status", v)
		}

		var resp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := client.Get(cmd.Context(), "/api/tasks/boards/"+args[0]+"/items", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Items, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NUMBER", Field: "number"},
			{Header: "TITLE", Field: "title", Width: 50},
			{Header: "STATUS", Field: "status"},
			{Header: "PRIORITY", Field: "priority"},
			{Header: "ASSIGNEE", Field: "assigneeId"},
		})
	},
}

var taskCreateCmd = &cobra.Command{
	Use:   "create <board-id>",
	Short: "Create a task",
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
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			overrides["status"] = v
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			overrides["priority"] = v
		}
		if cmd.Flags().Changed("assignee") {
			v, _ := cmd.Flags().GetString("assignee")
			overrides["assigneeId"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/tasks/boards/"+args[0]+"/items", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NUMBER", Field: "number"},
			{Header: "TITLE", Field: "title"},
		})
	},
}

var taskGetCmd = &cobra.Command{
	Use:   "get <item-id>",
	Short: "Get task details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/tasks/items/"+args[0], nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NUMBER", Field: "number"},
			{Header: "TITLE", Field: "title"},
			{Header: "DESCRIPTION", Field: "description", Width: 80},
			{Header: "STATUS", Field: "status"},
			{Header: "PRIORITY", Field: "priority"},
			{Header: "ASSIGNEE", Field: "assigneeId"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var taskEditCmd = &cobra.Command{
	Use:   "edit <item-id>",
	Short: "Edit a task",
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
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			overrides["status"] = v
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			overrides["priority"] = v
		}
		if cmd.Flags().Changed("assignee") {
			v, _ := cmd.Flags().GetString("assignee")
			overrides["assigneeId"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/tasks/items/"+args[0], body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
			{Header: "STATUS", Field: "status"},
		})
	},
}

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <item-id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete task %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/tasks/items/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Task deleted.")
		return nil
	},
}

var taskCommentCmd = &cobra.Command{
	Use:   "comment <item-id> <text>",
	Short: "Add a comment to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"content": args[1],
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/tasks/items/"+args[0]+"/comments", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Comment added.")
		return nil
	},
}

// --- Label subcommands ---

var taskLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage task labels",
}

var taskLabelListCmd = &cobra.Command{
	Use:   "list <item-id>",
	Short: "List labels on a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Labels []map[string]interface{} `json:"labels"`
		}
		if err := client.Get(cmd.Context(), "/api/tasks/items/"+args[0]+"/labels", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Labels, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "COLOR", Field: "color"},
		})
	},
}

var taskLabelCreateCmd = &cobra.Command{
	Use:   "create <item-id>",
	Short: "Add a label to a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			body["name"] = v
		}
		if cmd.Flags().Changed("color") {
			v, _ := cmd.Flags().GetString("color")
			body["color"] = v
		}
		if cmd.Flags().Changed("label-id") {
			v, _ := cmd.Flags().GetString("label-id")
			body["labelId"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/tasks/items/"+args[0]+"/labels", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Label added.")
		return nil
	},
}

var taskLabelEditCmd = &cobra.Command{
	Use:   "edit <item-id> <label-id>",
	Short: "Edit a label",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			body["name"] = v
		}
		if cmd.Flags().Changed("color") {
			v, _ := cmd.Flags().GetString("color")
			body["color"] = v
		}

		if err := client.Patch(cmd.Context(), "/api/tasks/items/"+args[0]+"/labels", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Label updated.")
		return nil
	},
}

var taskLabelDeleteCmd = &cobra.Command{
	Use:   "delete <item-id> <label-id>",
	Short: "Remove a label from a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"labelId": {args[1]}}

		if err := client.Get(cmd.Context(), "/api/tasks/items/"+args[0]+"/labels", q, nil); err != nil {
			return err
		}

		body := map[string]interface{}{
			"action":  "remove",
			"labelId": args[1],
		}
		if err := client.Post(cmd.Context(), "/api/tasks/items/"+args[0]+"/labels", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Label removed.")
		return nil
	},
}

func init() {
	cmdutil.AddListFlags(taskListCmd)
	taskListCmd.Flags().String("status", "", "Filter by status")

	taskCreateCmd.Flags().String("title", "", "Task title")
	taskCreateCmd.Flags().String("description", "", "Task description")
	taskCreateCmd.Flags().String("status", "", "Task status")
	taskCreateCmd.Flags().String("priority", "", "Task priority")
	taskCreateCmd.Flags().String("assignee", "", "Assignee actor ID")
	cmdutil.AddJSONInput(taskCreateCmd)

	taskEditCmd.Flags().String("title", "", "Task title")
	taskEditCmd.Flags().String("description", "", "Task description")
	taskEditCmd.Flags().String("status", "", "Task status")
	taskEditCmd.Flags().String("priority", "", "Task priority")
	taskEditCmd.Flags().String("assignee", "", "Assignee actor ID")
	cmdutil.AddJSONInput(taskEditCmd)

	taskLabelCreateCmd.Flags().String("name", "", "Label name")
	taskLabelCreateCmd.Flags().String("color", "", "Label color")
	taskLabelCreateCmd.Flags().String("label-id", "", "Existing label ID to assign")

	taskLabelEditCmd.Flags().String("name", "", "Label name")
	taskLabelEditCmd.Flags().String("color", "", "Label color")

	taskLabelCmd.AddCommand(taskLabelListCmd)
	taskLabelCmd.AddCommand(taskLabelCreateCmd)
	taskLabelCmd.AddCommand(taskLabelEditCmd)
	taskLabelCmd.AddCommand(taskLabelDeleteCmd)

	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskEditCmd)
	taskCmd.AddCommand(taskDeleteCmd)
	taskCmd.AddCommand(taskCommentCmd)
	taskCmd.AddCommand(taskLabelCmd)
}
