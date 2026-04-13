package cmd

import (
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var subscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Subscription and usage info",
}

var subscriptionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show subscription status",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/subscription", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "PLAN", Field: "plan"},
			{Header: "STATUS", Field: "status"},
			{Header: "CURRENT PERIOD END", Field: "currentPeriodEnd"},
			{Header: "BALANCE", Field: "balance"},
		})
	},
}

var subscriptionUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show current usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/usage/rate-limits", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "MESSAGES USED", Field: "messagesUsed"},
			{Header: "MESSAGES LIMIT", Field: "messagesLimit"},
			{Header: "STORAGE USED", Field: "storageUsed"},
			{Header: "STORAGE LIMIT", Field: "storageLimit"},
		})
	},
}

func init() {
	subscriptionCmd.AddCommand(subscriptionStatusCmd)
	subscriptionCmd.AddCommand(subscriptionUsageCmd)
}
