package cmd

import (
	"fmt"
	"strings"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/credential"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with idapt",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to idapt",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKey, _ := cmd.Flags().GetString("api-key")

		if apiKey != "" {
			if !strings.HasPrefix(apiKey, "uk_") && !strings.HasPrefix(apiKey, "ak_") && !strings.HasPrefix(apiKey, "pk_") && !strings.HasPrefix(apiKey, "mk_") {
				return fmt.Errorf("API key must start with uk_, ak_, pk_, or mk_")
			}
			creds := credential.Credentials{APIKey: apiKey}
			if err := credential.Save(credential.DefaultPath(), creds); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "API key saved successfully.")
			return nil
		}

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		if email == "" {
			return fmt.Errorf("--email is required (or use --api-key)")
		}
		if password == "" {
			return fmt.Errorf("--password is required")
		}

		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		err = client.Post(cmd.Context(), "/api/auth/sign-in/email", map[string]string{
			"email":    email,
			"password": password,
		}, &resp)
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := credential.Clear(credential.DefaultPath()); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		if f == nil {
			return fmt.Errorf("not logged in; use `idapt auth login`")
		}
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/auth/account", nil, &resp); err != nil {
			return fmt.Errorf("not authenticated: %w", err)
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, nil)
	},
}

func init() {
	authLoginCmd.Flags().String("api-key", "", "API key (uk_..., ak_..., pk_...)")
	authLoginCmd.Flags().String("email", "", "Email address")
	authLoginCmd.Flags().String("password", "", "Password")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}
