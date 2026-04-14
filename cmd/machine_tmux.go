package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineTmuxCmd = &cobra.Command{
	Use:   "tmux",
	Short: "Manage tmux sessions on a machine",
}

var machineTmuxListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name>",
	Short: "List tmux sessions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"command": "tmux list-sessions -F '#{session_name}\t#{session_windows}\t#{session_created}' 2>/dev/null || echo 'no sessions'",
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "OUTPUT", Field: "output"},
		})
	},
}

var machineTmuxRunCmd = &cobra.Command{
	Use:   "run <machine-id-or-name> <session-name> <command>",
	Short: "Run a command in a new tmux session",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		tmuxCmd := fmt.Sprintf("tmux new-session -d -s %s %q", args[1], args[2])
		body := map[string]interface{}{
			"command": tmuxCmd,
		}

		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Tmux session %s started.\n", args[1])
		return nil
	},
}

var machineTmuxCaptureCmd = &cobra.Command{
	Use:   "capture <machine-id-or-name> <session-name>",
	Short: "Capture output from a tmux session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		lines, _ := cmd.Flags().GetInt("lines")
		tmuxCmd := fmt.Sprintf("tmux capture-pane -t %s -p -S -%d", args[1], lines)
		body := map[string]interface{}{
			"command": tmuxCmd,
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, &resp); err != nil {
			return err
		}

		if out, ok := resp["output"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "OUTPUT", Field: "output"},
		})
	},
}

var machineTmuxSendCmd = &cobra.Command{
	Use:   "send <machine-id-or-name> <session-name> <keys>",
	Short: "Send keys to a tmux session",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		tmuxCmd := fmt.Sprintf("tmux send-keys -t %s %q Enter", args[1], args[2])
		body := map[string]interface{}{
			"command": tmuxCmd,
		}

		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, nil); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Keys sent.")
		return nil
	},
}

var machineTmuxKillCmd = &cobra.Command{
	Use:   "kill <machine-id-or-name> <session-name>",
	Short: "Kill a tmux session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		tmuxCmd := fmt.Sprintf("tmux kill-session -t %s", args[1])
		body := map[string]interface{}{
			"command": tmuxCmd,
		}

		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/terminal", body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session %s killed.\n", args[1])
		return nil
	},
}

func init() {
	machineTmuxCaptureCmd.Flags().Int("lines", 100, "Number of lines to capture")

	machineTmuxCmd.AddCommand(machineTmuxListCmd)
	machineTmuxCmd.AddCommand(machineTmuxRunCmd)
	machineTmuxCmd.AddCommand(machineTmuxCaptureCmd)
	machineTmuxCmd.AddCommand(machineTmuxSendCmd)
	machineTmuxCmd.AddCommand(machineTmuxKillCmd)
}
