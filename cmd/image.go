package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

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

		// Project (required)
		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			projectID = f.Config.DefaultProject
		}
		if projectID == "" {
			return fmt.Errorf("--project flag or default project is required")
		}
		body["projectId"] = projectID

		// Model
		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			body["model"] = v
		}

		// Output path
		if cmd.Flags().Changed("output") {
			v, _ := cmd.Flags().GetString("output")
			body["outputPath"] = v
		}

		// Input images: classify into 3 types
		inputPaths, _ := cmd.Flags().GetStringSlice("input")
		if len(inputPaths) > 0 {
			var inputImages []string     // base64 data URLs (local files)
			var inputImageUrls []string  // public URLs
			var inputImagePaths []string // remote idapt project paths

			for _, p := range inputPaths {
				switch classifyInput(p) {
				case inputTypeURL:
					inputImageUrls = append(inputImageUrls, p)
				case inputTypeRemotePath:
					inputImagePaths = append(inputImagePaths, p)
				case inputTypeLocalFile:
					data, readErr := os.ReadFile(p)
					if readErr != nil {
						return fmt.Errorf("failed to read input file %q: %w", p, readErr)
					}
					b64 := base64.StdEncoding.EncodeToString(data)
					inputImages = append(inputImages, "data:image/png;base64,"+b64)
				}
			}

			if len(inputImages) > 0 {
				body["inputImages"] = inputImages
			}
			if len(inputImageUrls) > 0 {
				body["inputImageUrls"] = inputImageUrls
			}
			if len(inputImagePaths) > 0 {
				body["inputImagePaths"] = inputImagePaths
			}
		}

		// Size (legacy, may be ignored by new API)
		if cmd.Flags().Changed("size") {
			v, _ := cmd.Flags().GetString("size")
			body["size"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/image/generate", body, &resp); err != nil {
			return err
		}

		// Print URL directly if available
		if u, ok := resp["url"].(string); ok && u != "" {
			fmt.Fprintln(cmd.OutOrStdout(), u)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "URL", Field: "url"},
			{Header: "MODEL", Field: "model"},
			{Header: "PATH", Field: "path"},
			{Header: "COST", Field: "costUsd"},
		})
	},
}

var imageListModelsCmd = &cobra.Command{
	Use:   "list-models",
	Short: "List available image generation models",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Models []map[string]interface{} `json:"models"`
		}
		if err := client.Get(cmd.Context(), "/api/image/models", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Models, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "displayName"},
			{Header: "PROVIDER", Field: "providerDisplayName"},
			{Header: "COST/IMAGE", Field: "costPerImage"},
			{Header: "SPEED", Field: "speed"},
		})
	},
}

// inputType classifies --input values into 3 categories.
type inputType int

const (
	inputTypeLocalFile  inputType = iota
	inputTypeURL        inputType = 1
	inputTypeRemotePath inputType = 2
)

// classifyInput determines whether an --input value is a URL, remote idapt path, or local file.
func classifyInput(value string) inputType {
	// URLs
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return inputTypeURL
	}

	// Remote idapt paths look like absolute paths with at least 3 segments:
	// /owner/project/files/... or similar
	if strings.HasPrefix(value, "/") && strings.Count(value, "/") >= 3 {
		return inputTypeRemotePath
	}

	// Everything else is a local file
	return inputTypeLocalFile
}

func init() {
	imageGenerateCmd.Flags().String("model", "", "Image generation model ID")
	imageGenerateCmd.Flags().String("size", "", "Image size (e.g. 1024x1024)")
	imageGenerateCmd.Flags().String("output", "", "Output path in project (e.g. 'Generated Images/sunset.png')")
	imageGenerateCmd.Flags().String("project", "", "Project ID")
	imageGenerateCmd.Flags().StringSlice("input", nil, "Input image paths (local files, URLs, or remote idapt paths)")

	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageListModelsCmd)
}
