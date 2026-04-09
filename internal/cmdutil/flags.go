package cmdutil

import (
	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	APIKey    string
	Project   string
	APIURL    string
	Output    string
	NoColor   bool
	Verbose   bool
	Confirm   bool
	JSONInput string
}

func RegisterGlobalFlags(root *cobra.Command) *GlobalFlags {
	f := &GlobalFlags{}
	root.PersistentFlags().StringVar(&f.APIKey, "api-key", "", "API key for authentication")
	root.PersistentFlags().StringVar(&f.Project, "project", "", "Project slug or ID")
	root.PersistentFlags().StringVar(&f.APIURL, "api-url", "", "API base URL (default https://idapt.ai)")
	root.PersistentFlags().StringVarP(&f.Output, "output", "o", "", "Output format: table|json|jsonl|quiet")
	root.PersistentFlags().BoolVar(&f.NoColor, "no-color", false, "Disable color output")
	root.PersistentFlags().BoolVar(&f.Verbose, "verbose", false, "Verbose output")
	root.PersistentFlags().BoolVar(&f.Confirm, "confirm", false, "Skip confirmation prompts")
	return f
}

func AddListFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", 50, "Maximum items to return")
	cmd.Flags().String("starting-after", "", "Cursor for next page")
}

func AddJSONInput(cmd *cobra.Command) {
	cmd.Flags().String("json", "", "JSON input (inline or - for stdin)")
}
