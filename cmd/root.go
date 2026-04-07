package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

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
		if isDaemonCommand(cmd) {
			return nil
		}

		var cfg cliconfig.Config
		if cfgPath, err := cliconfig.DefaultPath(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v (config file unavailable)\n", err)
			cfg = cliconfig.Defaults()
		} else if loaded, err := cliconfig.Load(cfgPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load config: %v\n", err)
			cfg = cliconfig.Defaults()
		} else {
			cfg = loaded
		}

		var creds credential.Credentials
		var credPath string
		if cp, err := credential.DefaultPath(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v (credentials file unavailable)\n", err)
		} else {
			credPath = cp
			if loaded, err := credential.Load(credPath); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load credentials: %v\n", err)
			} else {
				creds = loaded
			}
		}

		apiKey := globalFlags.APIKey
		apiKeySource := ""
		if apiKey != "" {
			apiKeySource = "flag"
		}
		if apiKey == "" {
			envKey := os.Getenv("IDAPT_API_KEY")
			if envKey != "" && !strings.HasPrefix(envKey, "mk_") {
				apiKey = envKey
				apiKeySource = "env"
			}
		}
		if apiKey == "" {
			apiKey = creds.APIKey
			if apiKey != "" {
				apiKeySource = "file"
			}
		}

		if globalFlags.Verbose && apiKey != "" {
			masked := apiKey[:min(len(apiKey), 6)] + "..."
			fmt.Fprintf(cmd.ErrOrStderr(), "Auth: using %s from %s\n", masked, apiKeySource)
		}
		if globalFlags.Verbose && apiKey == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Auth: no API key found (flag=%v, env=%v, file=%q, fileKey=%v)\n",
				globalFlags.APIKey != "", os.Getenv("IDAPT_API_KEY") != "", credPath, creds.APIKey != "")
		}

		if apiKey == "" && os.Geteuid() == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: no API key found. If using sudo, pass --api-key or use sudo -E to preserve IDAPT_API_KEY.")
		}

		apiURL := globalFlags.APIURL
		if apiURL == "" {
			apiURL = cfg.APIURL
		}
		if globalFlags.Verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "API URL: %s\n", apiURL)
		}

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

func NewRootCmd() *cobra.Command {
	root := rootCmd
	root.SetContext(context.Background())
	return root
}

func Execute() error {
	rootCmd.SetContext(context.Background())
	return rootCmd.Execute()
}

func init() {
	globalFlags = cmdutil.RegisterGlobalFlags(rootCmd)

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCliCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(fileCmd)
	rootCmd.AddCommand(machineRemoteCmd)
	rootCmd.AddCommand(scriptCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(mediaCmd)
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
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		pn := p.Name()
		if pn == "firewall" || pn == "proxy" {
			return true
		}
	}
	if name == "firewall" || name == "proxy" {
		return true
	}
	return false
}
