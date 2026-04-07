package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute code or bash commands via Lambda",
}

var execCodeCmd = &cobra.Command{
	Use:   "code <language>",
	Short: "Execute code in a sandboxed environment",
	Long:  "Executes code via Lambda. Pass code via --code flag or --file flag.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"language": args[0],
		}

		if cmd.Flags().Changed("code") {
			v, _ := cmd.Flags().GetString("code")
			body["code"] = v
		}
		if cmd.Flags().Changed("file") {
			path, _ := cmd.Flags().GetString("file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return fmt.Errorf("reading code file: %w", err)
			}
			body["code"] = content
		}

		if _, ok := body["code"]; !ok {
			return fmt.Errorf("--code or --file is required")
		}

		if cmd.Flags().Changed("timeout") {
			v, _ := cmd.Flags().GetInt("timeout")
			body["timeout"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/exec/code", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
			if stderr, ok := resp["stderr"].(string); ok && stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), stderr)
			}
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "EXIT CODE", Field: "exitCode"},
			{Header: "OUTPUT", Field: "output"},
			{Header: "STDERR", Field: "stderr"},
		})
	},
}

var execBashCmd = &cobra.Command{
	Use:   "bash <command>",
	Short: "Execute a bash command via Lambda",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		command := args[0]
		if len(args) > 1 {
			command = ""
			for i, arg := range args {
				if i > 0 {
					command += " "
				}
				command += arg
			}
		}

		body := map[string]interface{}{
			"command": command,
		}

		if cmd.Flags().Changed("timeout") {
			v, _ := cmd.Flags().GetInt("timeout")
			body["timeout"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/exec/bash", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
			if stderr, ok := resp["stderr"].(string); ok && stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), stderr)
			}
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "EXIT CODE", Field: "exitCode"},
			{Header: "OUTPUT", Field: "output"},
			{Header: "STDERR", Field: "stderr"},
		})
	},
}

func init() {
	execCodeCmd.Flags().String("code", "", "Code to execute")
	execCodeCmd.Flags().String("file", "", "Path to code file")
	execCodeCmd.Flags().Int("timeout", 0, "Timeout in seconds")

	execBashCmd.Flags().Int("timeout", 0, "Timeout in seconds")

	execCmd.AddCommand(execCodeCmd)
	execCmd.AddCommand(execBashCmd)
}
