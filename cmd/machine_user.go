package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var machineUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage machine users",
}

var machineUserListCmd = &cobra.Command{
	Use:   "list <machine-id-or-name>",
	Short: "List users on a machine",
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

		var resp struct {
			Users []map[string]interface{} `json:"users"`
		}
		if err := client.Get(cmd.Context(), "/api/machines/"+id+"/users", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Users, []output.Column{
			{Header: "USERNAME", Field: "username"},
			{Header: "SHELL", Field: "shell"},
			{Header: "HOME", Field: "home"},
			{Header: "SUDO", Field: "sudo"},
		})
	},
}

var machineUserCreateCmd = &cobra.Command{
	Use:   "create <machine-id-or-name>",
	Short: "Create a user on a machine",
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
		if cmd.Flags().Changed("username") {
			v, _ := cmd.Flags().GetString("username")
			overrides["username"] = v
		}
		if cmd.Flags().Changed("password") {
			v, _ := cmd.Flags().GetString("password")
			overrides["password"] = v
		}
		if cmd.Flags().Changed("shell") {
			v, _ := cmd.Flags().GetString("shell")
			overrides["shell"] = v
		}
		if cmd.Flags().Changed("sudo") {
			v, _ := cmd.Flags().GetBool("sudo")
			overrides["sudo"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/machines/"+id+"/users", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "User created.")
		return nil
	},
}

var machineUserDeleteCmd = &cobra.Command{
	Use:   "delete <machine-id-or-name> <username>",
	Short: "Delete a user from a machine",
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

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete user %s?", args[1])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/machines/"+id+"/users/"+args[1]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "User %s deleted.\n", args[1])
		return nil
	},
}

var machineUserEditCmd = &cobra.Command{
	Use:   "edit <machine-id-or-name> <username>",
	Short: "Edit a user on a machine",
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

		body := map[string]interface{}{}
		if cmd.Flags().Changed("password") {
			v, _ := cmd.Flags().GetString("password")
			body["password"] = v
		}
		if cmd.Flags().Changed("shell") {
			v, _ := cmd.Flags().GetString("shell")
			body["shell"] = v
		}
		if cmd.Flags().Changed("sudo") {
			v, _ := cmd.Flags().GetBool("sudo")
			body["sudo"] = v
		}

		if err := client.Patch(cmd.Context(), "/api/machines/"+id+"/users/"+args[1], body, nil); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "User %s updated.\n", args[1])
		return nil
	},
}

func init() {
	machineUserCreateCmd.Flags().String("username", "", "Username")
	machineUserCreateCmd.Flags().String("password", "", "Password")
	machineUserCreateCmd.Flags().String("shell", "/bin/bash", "Login shell")
	machineUserCreateCmd.Flags().Bool("sudo", false, "Grant sudo access")
	cmdutil.AddJSONInput(machineUserCreateCmd)

	machineUserEditCmd.Flags().String("password", "", "New password")
	machineUserEditCmd.Flags().String("shell", "", "Login shell")
	machineUserEditCmd.Flags().Bool("sudo", false, "Grant sudo access")

	machineUserCmd.AddCommand(machineUserListCmd)
	machineUserCmd.AddCommand(machineUserCreateCmd)
	machineUserCmd.AddCommand(machineUserDeleteCmd)
	machineUserCmd.AddCommand(machineUserEditCmd)
}
