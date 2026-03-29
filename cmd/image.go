package cmd

import (
	"fmt"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Image generation",
}

var imageGenerateCmd = &cobra.Command{
	Use:   "generate <prompt>",
	Short: "Generate an image from a text prompt",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"prompt": args[0],
		}

		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			body["model"] = v
		}
		if cmd.Flags().Changed("size") {
			v, _ := cmd.Flags().GetString("size")
			body["size"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/image/generate", body, &resp); err != nil {
			return err
		}

		// Print URL directly
		if u, ok := resp["url"].(string); ok {
			fmt.Fprintln(cmd.OutOrStdout(), u)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "URL", Field: "url"},
			{Header: "MODEL", Field: "model"},
		})
	},
}

func init() {
	imageGenerateCmd.Flags().String("model", "", "Image generation model")
	imageGenerateCmd.Flags().String("size", "", "Image size (e.g. 1024x1024)")

	imageCmd.AddCommand(imageGenerateCmd)
}
