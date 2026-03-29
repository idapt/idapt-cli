package cmd

import (
	"fmt"
	"strings"

	"github.com/idapt/idapt-cli/internal/cliconfig"
	"github.com/spf13/cobra"
)

var configCliCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := cliconfig.DefaultPath()
		cfg, err := cliconfig.Load(path)
		if err != nil {
			cfg = cliconfig.Defaults()
		}

		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}

		if err := cliconfig.Save(path, cfg); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cliconfig.Load(cliconfig.DefaultPath())
		if err != nil {
			return err
		}

		if len(args) == 0 {
			// Show all
			for _, key := range cliconfig.Keys() {
				val, _ := cfg.Get(key)
				fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, val)
			}
			return nil
		}

		val, ok := cfg.Get(args[0])
		if !ok {
			return fmt.Errorf("unknown config key %q; valid keys: %s", args[0], strings.Join(cliconfig.Keys(), ", "))
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cliconfig.Load(cliconfig.DefaultPath())
		if err != nil {
			return err
		}
		for _, key := range cliconfig.Keys() {
			val, _ := cfg.Get(key)
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, val)
		}
		return nil
	},
}

func init() {
	configCliCmd.AddCommand(configSetCmd)
	configCliCmd.AddCommand(configGetCmd)
	configCliCmd.AddCommand(configListCmd)
}
