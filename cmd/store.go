package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Browse and install from the store",
}

var storeSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the store for all resource types",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		if cmd.Flags().Changed("type") {
			v, _ := cmd.Flags().GetString("type")
			q.Set("type", v)
		}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/explore/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TYPE", Field: "type"},
			{Header: "NAME", Field: "name"},
			{Header: "DESCRIPTION", Field: "description", Width: 60},
			{Header: "AUTHOR", Field: "authorName"},
		})
	},
}

// --- Skill store ---

var storeSkillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Browse and install skills from the store",
}

var storeSkillSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search skills in the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		q = buildListQuery(cmd, q)

		var resp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := client.Get(cmd.Context(), "/api/skill-store", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Items, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "DESCRIPTION", Field: "description", Width: 60},
			{Header: "INSTALLS", Field: "installCount"},
		})
	},
}

var storeSkillInstallCmd = &cobra.Command{
	Use:   "install <resource-id>",
	Short: "Install a skill from the store",
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

		body := map[string]interface{}{"projectId": projectID}
		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/skill-store/"+args[0]+"/install", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Skill installed.")
		return nil
	},
}

// --- KB store ---

var storeKBCmd = &cobra.Command{
	Use:   "kb",
	Short: "Browse and install knowledge bases from the store",
}

var storeKBSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search knowledge bases in the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		q = buildListQuery(cmd, q)

		var resp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := client.Get(cmd.Context(), "/api/kb-store", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Items, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "DESCRIPTION", Field: "description", Width: 60},
			{Header: "INSTALLS", Field: "installCount"},
		})
	},
}

var storeKBInstallCmd = &cobra.Command{
	Use:   "install <resource-id>",
	Short: "Install a knowledge base from the store",
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

		body := map[string]interface{}{"projectId": projectID}
		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/kb-store/"+args[0]+"/install", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Knowledge base installed.")
		return nil
	},
}

// --- Script store ---

var storeScriptCmd = &cobra.Command{
	Use:   "script",
	Short: "Browse and install scripts from the store",
}

var storeScriptSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search scripts in the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		q = buildListQuery(cmd, q)

		var resp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := client.Get(cmd.Context(), "/api/script-store", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Items, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "DESCRIPTION", Field: "description", Width: 60},
			{Header: "INSTALLS", Field: "installCount"},
		})
	},
}

var storeScriptInstallCmd = &cobra.Command{
	Use:   "install <resource-id>",
	Short: "Install a script from the store",
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

		body := map[string]interface{}{"projectId": projectID}
		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/script-store/"+args[0]+"/install", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Script installed.")
		return nil
	},
}

// --- Agent store ---

var storeAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Browse and install agents from the store",
}

var storeAgentSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search agents in the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[0]}}
		q = buildListQuery(cmd, q)

		var resp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := client.Get(cmd.Context(), "/api/agent-store", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Items, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "DESCRIPTION", Field: "description", Width: 60},
			{Header: "INSTALLS", Field: "installCount"},
		})
	},
}

var storeAgentInstallCmd = &cobra.Command{
	Use:   "install <resource-id>",
	Short: "Install an agent from the store",
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

		body := map[string]interface{}{"projectId": projectID}
		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/agent-store/"+args[0]+"/install", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Agent installed.")
		return nil
	},
}

func init() {
	storeSearchCmd.Flags().String("type", "", "Filter by resource type (skill, kb, script, agent)")

	cmdutil.AddListFlags(storeSkillSearchCmd)
	cmdutil.AddListFlags(storeKBSearchCmd)
	cmdutil.AddListFlags(storeScriptSearchCmd)
	cmdutil.AddListFlags(storeAgentSearchCmd)

	storeSkillCmd.AddCommand(storeSkillSearchCmd)
	storeSkillCmd.AddCommand(storeSkillInstallCmd)

	storeKBCmd.AddCommand(storeKBSearchCmd)
	storeKBCmd.AddCommand(storeKBInstallCmd)

	storeScriptCmd.AddCommand(storeScriptSearchCmd)
	storeScriptCmd.AddCommand(storeScriptInstallCmd)

	storeAgentCmd.AddCommand(storeAgentSearchCmd)
	storeAgentCmd.AddCommand(storeAgentInstallCmd)

	storeCmd.AddCommand(storeSearchCmd)
	storeCmd.AddCommand(storeSkillCmd)
	storeCmd.AddCommand(storeKBCmd)
	storeCmd.AddCommand(storeScriptCmd)
	storeCmd.AddCommand(storeAgentCmd)
}
