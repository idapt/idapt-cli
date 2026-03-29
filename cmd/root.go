package cmd

import (
	"context"
	"os"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cliconfig"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/credential"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var globalFlags *cmdutil.GlobalFlags

var rootCmd = &cobra.Command{
	Use:   "idapt",
	Short: "idapt CLI — AI workspace from the terminal",
	Long:  "idapt is a CLI tool and per-machine daemon for the idapt platform.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip factory setup for daemon commands and help
		if isDaemonCommand(cmd) {
			return nil
		}

		cfg, _ := cliconfig.Load(cliconfig.DefaultPath())
		creds, _ := credential.Load(credential.DefaultPath())

		// Resolve API key: flag > env > credential file
		apiKey := globalFlags.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("IDAPT_API_KEY")
		}
		if apiKey == "" {
			apiKey = creds.APIKey
		}

		// Resolve API URL: flag > env > config > default
		apiURL := globalFlags.APIURL
		if apiURL == "" {
			apiURL = cfg.APIURL
		}

		// Resolve output format: flag > config > auto-detect
		format := output.Format(globalFlags.Output)
		if format == "" {
			format = output.Format(cfg.OutputFormat)
		}
		if format == "" {
			format = output.Detect()
		}

		noColor := globalFlags.NoColor || cfg.NoColor

		f := &cmdutil.Factory{
			Config:      cfg,
			Credentials: creds,
			Format:      format,
			NoColor:     noColor,
			Out:         cmd.OutOrStdout(),
			ErrOut:      cmd.ErrOrStderr(),
			In:          cmd.InOrStdin(),
		}
		f.SetClientFn(func() (*api.Client, error) {
			c, err := api.NewClient(api.ClientConfig{
				BaseURL:    apiURL,
				APIKey:     apiKey,
				Verbose:    globalFlags.Verbose,
				CLIVersion: Version,
			})
			if err != nil {
				return nil, err
			}
			if globalFlags.Verbose {
				c.SetErrOut(cmd.ErrOrStderr())
			}
			return c, nil
		})

		cmdutil.SetFactory(cmd, f)
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

// NewRootCmd creates a fresh root command (for testing).
func NewRootCmd() *cobra.Command {
	root := rootCmd
	root.SetContext(context.Background())
	return root
}

// Execute runs the root command.
func Execute() error {
	rootCmd.SetContext(context.Background())
	return rootCmd.Execute()
}

func init() {
	globalFlags = cmdutil.RegisterGlobalFlags(rootCmd)

	// Existing daemon commands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	// firewallCmd and proxyCmd add themselves via their own init()

	// User-facing commands
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCliCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(fileCmd)
	rootCmd.AddCommand(kbCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(machineRemoteCmd)
	rootCmd.AddCommand(scriptCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(subscriptionCmd)
	rootCmd.AddCommand(apikeyCmd)
	rootCmd.AddCommand(shareCmd)
	rootCmd.AddCommand(notificationCmd)
	rootCmd.AddCommand(multiAgentCmd)
}

func isDaemonCommand(cmd *cobra.Command) bool {
	name := cmd.Name()
	if name == "version" || name == "serve" || name == "help" {
		return true
	}
	// Check parent chain for daemon commands
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		pn := p.Name()
		if pn == "firewall" || pn == "proxy" {
			return true
		}
	}
	// Direct child of firewall/proxy
	if name == "firewall" || name == "proxy" {
		return true
	}
	return false
}
