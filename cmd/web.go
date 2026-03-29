package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Web search and fetch",
}

var webSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the web",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		if cmd.Flags().Changed("limit") {
			limit, _ := cmd.Flags().GetInt("limit")
			q.Set("limit", fmt.Sprintf("%d", limit))
		}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/web/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "TITLE", Field: "title", Width: 50},
			{Header: "URL", Field: "url", Width: 60},
			{Header: "SNIPPET", Field: "snippet", Width: 80},
		})
	},
}

var webFetchCmd = &cobra.Command{
	Use:   "fetch <url>",
	Short: "Fetch and extract content from a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"url": args[0],
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/web/fetch", body, &resp); err != nil {
			return err
		}

		// Print content directly if in text mode
		if content, ok := resp["content"].(string); ok {
			fmt.Fprintln(cmd.OutOrStdout(), content)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "TITLE", Field: "title"},
			{Header: "CONTENT", Field: "content", Width: 120},
		})
	},
}

func init() {
	webSearchCmd.Flags().Int("limit", 10, "Number of results")

	webCmd.AddCommand(webSearchCmd)
	webCmd.AddCommand(webFetchCmd)
}
