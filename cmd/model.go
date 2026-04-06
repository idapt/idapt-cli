package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Browse and manage models",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{}
		if cmd.Flags().Changed("provider") {
			v, _ := cmd.Flags().GetString("provider")
			q.Set("provider", v)
		}

		var resp struct {
			Models []map[string]interface{} `json:"models"`
		}
		if err := client.Get(cmd.Context(), "/api/models", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Models, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "PROVIDER", Field: "provider"},
			{Header: "CONTEXT", Field: "contextLength"},
			{Header: "COST/1K IN", Field: "inputCostPer1k"},
			{Header: "COST/1K OUT", Field: "outputCostPer1k"},
		})
	},
}

var modelSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search models",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}

		var resp struct {
			Models []map[string]interface{} `json:"models"`
		}
		if err := client.Get(cmd.Context(), "/api/models", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Models, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "PROVIDER", Field: "provider"},
		})
	},
}
var modelFavoriteCmd = &cobra.Command{
	Use:   "favorite",
	Short: "Manage model favorites",
}

var modelFavoriteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List favorite models",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Favorites []map[string]interface{} `json:"favorites"`
		}
		if err := client.Get(cmd.Context(), "/api/model-favorites", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Favorites, []output.Column{
			{Header: "MODEL ID", Field: "modelId"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var modelFavoriteAddCmd = &cobra.Command{
	Use:   "add <model-id>",
	Short: "Add a model to favorites",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"modelId": args[0],
		}

		if err := client.Post(cmd.Context(), "/api/model-favorites", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Model %s added to favorites.\n", args[0])
		return nil
	},
}

var modelFavoriteRemoveCmd = &cobra.Command{
	Use:   "remove <model-id>",
	Short: "Remove a model from favorites",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"modelId": args[0],
			"action":  "remove",
		}

		if err := client.Post(cmd.Context(), "/api/model-favorites", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Model %s removed from favorites.\n", args[0])
		return nil
	},
}

func init() {
	modelListCmd.Flags().String("provider", "", "Filter by provider (openai, anthropic, google, etc.)")

	modelFavoriteCmd.AddCommand(modelFavoriteListCmd)
	modelFavoriteCmd.AddCommand(modelFavoriteAddCmd)
	modelFavoriteCmd.AddCommand(modelFavoriteRemoveCmd)

	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelSearchCmd)
	modelCmd.AddCommand(modelFavoriteCmd)
}
