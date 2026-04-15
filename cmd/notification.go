package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var notificationCmd = &cobra.Command{
	Use:     "notification",
	Aliases: []string{"notif"},
	Short:   "Manage notifications",
}

var notificationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Notifications []map[string]interface{} `json:"notifications"`
		}
		if err := client.Get(cmd.Context(), "/api/notifications", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Notifications, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "type"},
			{Header: "MESSAGE", Field: "message", Width: 60},
			{Header: "READ", Field: "read"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var notificationReadCmd = &cobra.Command{
	Use:   "read [id]",
	Short: "Mark notification(s) as read",
	Long:  "Mark a specific notification as read, or all notifications if no ID is provided.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if len(args) > 0 {
			body := map[string]interface{}{"read": true}
			if err := client.Patch(cmd.Context(), "/api/notifications/"+args[0], body, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Notification marked as read.")
		} else {
			body := map[string]interface{}{"readAll": true}
			if err := client.Post(cmd.Context(), "/api/notifications/read-all", body, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "All notifications marked as read.")
		}

		return nil
	},
}

func init() {
	notificationCmd.AddCommand(notificationListCmd)
	notificationCmd.AddCommand(notificationReadCmd)
}
