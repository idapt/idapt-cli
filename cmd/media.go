package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)
type inputType int

const (
	inputTypeLocalFile  inputType = iota
	inputTypeURL        inputType = 1
	inputTypeRemotePath inputType = 2
)

func classifyInput(value string) inputType {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return inputTypeURL
	}
	if strings.HasPrefix(value, "/") && strings.Count(value, "/") >= 3 {
		return inputTypeRemotePath
	}
	return inputTypeLocalFile
}
var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Media operations (image generation, text-to-speech, audio transcription)",
}
var mediaGenerateCmd = &cobra.Command{
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

		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			projectID = f.Config.DefaultProject
		}
		if projectID == "" {
			return fmt.Errorf("--project flag or default project is required")
		}
		body["projectId"] = projectID

		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			body["model"] = v
		}
		if cmd.Flags().Changed("output") {
			v, _ := cmd.Flags().GetString("output")
			body["outputPath"] = v
		}
		if cmd.Flags().Changed("size") {
			v, _ := cmd.Flags().GetString("size")
			body["size"] = v
		}

		if cmd.Flags().Changed("input") {
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
						ext := strings.ToLower(filepath.Ext(p))
						mimeType := mime.TypeByExtension(ext)
						if mimeType == "" {
							mimeType = "image/png"
						}
						inputImages = append(inputImages, fmt.Sprintf("data:%s;base64,%s", mimeType, b64))
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
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/image/generate", body, &resp); err != nil {
			return err
		}

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
var mediaListModelsCmd = &cobra.Command{
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
var mediaTranscribeCmd = &cobra.Command{
	Use:   "transcribe <file-path-or-url>",
	Short: "Transcribe an audio file or URL to text",
	Long:  "Transcribe a local audio file or a remote URL to text. Accepts local file paths and http/https URLs.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		source := args[0]
		isURL := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")

		var audioReader io.Reader
		var filename string
		var mimeType string

		if isURL {
			resp, dlErr := http.Get(source)
			if dlErr != nil {
				return fmt.Errorf("failed to download: %w", dlErr)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("URL returned HTTP %d", resp.StatusCode)
			}
			audioReader = resp.Body
			parts := strings.Split(strings.TrimRight(source, "/"), "/")
			filename = parts[len(parts)-1]
			if filename == "" {
				filename = "audio.mp3"
			}
			mimeType = strings.Split(resp.Header.Get("Content-Type"), ";")[0]
			if mimeType == "" || mimeType == "application/octet-stream" {
				mimeType = mime.TypeByExtension(filepath.Ext(filename))
			}
		} else {
			file, openErr := os.Open(source)
			if openErr != nil {
				return fmt.Errorf("cannot open file: %w", openErr)
			}
			defer file.Close()
			audioReader = file
			filename = filepath.Base(source)
			mimeType = mime.TypeByExtension(filepath.Ext(source))
		}

		if mimeType == "" {
			mimeType = "audio/mpeg"
		}

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			_ = writer.WriteField("model", v)
		}
		if cmd.Flags().Changed("language") {
			v, _ := cmd.Flags().GetString("language")
			_ = writer.WriteField("language", v)
		}

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
		h.Set("Content-Type", mimeType)

		part, err := writer.CreatePart(h)
		if err != nil {
			return fmt.Errorf("creating multipart part: %w", err)
		}
		if _, err := io.Copy(part, audioReader); err != nil {
			return fmt.Errorf("writing file to multipart: %w", err)
		}
		writer.Close()

		httpResp, err := client.Do(cmd.Context(), "POST", "/api/v1/audio/transcriptions", &buf,
			api.WithHeader("Content-Type", writer.FormDataContentType()),
		)
		if err != nil {
			return err
		}
		defer httpResp.Body.Close()

		var resp map[string]interface{}
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		if text, ok := resp["text"].(string); ok {
			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath != "" {
				if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Transcription saved to %s\n", outputPath)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), text)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "TEXT", Field: "text"},
		})
	},
}
var mediaSpeakCmd = &cobra.Command{
	Use:   "speak <text>",
	Short: "Generate speech from text",
	Long:  "Generate speech from text. Text can be provided directly, from stdin with '-', or from a file with '@path'.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		text := args[0]
		if text == "-" {
			data, readErr := io.ReadAll(f.In)
			if readErr != nil {
				return fmt.Errorf("reading stdin: %w", readErr)
			}
			text = strings.TrimSpace(string(data))
		} else if strings.HasPrefix(text, "@") {
			filePath := strings.TrimPrefix(text, "@")
			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return fmt.Errorf("reading file %q: %w", filePath, readErr)
			}
			text = strings.TrimSpace(string(data))
		}

		if text == "" {
			return fmt.Errorf("text is empty")
		}

		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			projectID = f.Config.DefaultProject
		}
		if projectID == "" {
			return fmt.Errorf("--project flag or default project is required")
		}

		body := map[string]interface{}{
			"text":      text,
			"projectId": projectID,
		}

		if cmd.Flags().Changed("voice") {
			v, _ := cmd.Flags().GetString("voice")
			body["voiceId"] = v
		}
		if cmd.Flags().Changed("model") {
			v, _ := cmd.Flags().GetString("model")
			body["modelId"] = v
		}
		if cmd.Flags().Changed("speed") {
			v, _ := cmd.Flags().GetFloat64("speed")
			body["speed"] = v
		}
		if cmd.Flags().Changed("pitch") {
			v, _ := cmd.Flags().GetInt("pitch")
			body["pitch"] = v
		}
		if cmd.Flags().Changed("emotion") {
			v, _ := cmd.Flags().GetString("emotion")
			body["emotion"] = v
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/audio/generate", body, &resp); err != nil {
			return err
		}

		audioURL, _ := resp["url"].(string)

		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath != "" && audioURL != "" {
			dlResp, dlErr := http.Get(audioURL)
			if dlErr != nil {
				return fmt.Errorf("downloading audio: %w", dlErr)
			}
			defer dlResp.Body.Close()
			if dlResp.StatusCode != http.StatusOK {
				return fmt.Errorf("download returned HTTP %d", dlResp.StatusCode)
			}

			out, createErr := os.Create(outputPath)
			if createErr != nil {
				return fmt.Errorf("creating output file: %w", createErr)
			}
			defer out.Close()

			n, copyErr := io.Copy(out, dlResp.Body)
			if copyErr != nil {
				return fmt.Errorf("writing audio file: %w", copyErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Audio saved to %s (%d bytes).\n", outputPath, n)
			return nil
		}

		if audioURL != "" {
			fmt.Fprintln(cmd.OutOrStdout(), audioURL)
			return nil
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "URL", Field: "url"},
			{Header: "MODEL", Field: "modelId"},
			{Header: "COST", Field: "costUsd"},
		})
	},
}
var mediaListVoicesCmd = &cobra.Command{
	Use:   "list-voices",
	Short: "List available text-to-speech voices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		query := url.Values{}
		if cmd.Flags().Changed("language") {
			v, _ := cmd.Flags().GetString("language")
			query.Set("language", v)
		}
		if cmd.Flags().Changed("gender") {
			v, _ := cmd.Flags().GetString("gender")
			query.Set("gender", v)
		}

		var resp struct {
			Voices []map[string]interface{} `json:"voices"`
		}
		if err := client.Get(cmd.Context(), "/api/audio/voices", query, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Voices, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "GENDER", Field: "gender"},
			{Header: "LANGUAGE", Field: "language"},
			{Header: "CATEGORY", Field: "category"},
		})
	},
}
var mediaListTTSModelsCmd = &cobra.Command{
	Use:   "list-tts-models",
	Short: "List available text-to-speech models",
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
		if err := client.Get(cmd.Context(), "/api/audio/models", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Models, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "COST/1K CHARS", Field: "costPer1kChars"},
			{Header: "SPEED", Field: "speed"},
		})
	},
}
func init() {
	mediaGenerateCmd.Flags().String("model", "", "Image generation model ID")
	mediaGenerateCmd.Flags().String("size", "", "Image size (e.g. 1024x1024)")
	mediaGenerateCmd.Flags().String("output", "", "Output path in project (e.g. 'Generated Images/sunset.png')")
	mediaGenerateCmd.Flags().String("project", "", "Project ID")
	mediaGenerateCmd.Flags().StringSlice("input", nil, "Input image paths (local files, URLs, or remote idapt paths)")

	mediaTranscribeCmd.Flags().String("model", "", "Transcription model (gpt-4o-mini-transcribe or gpt-4o-transcribe)")
	mediaTranscribeCmd.Flags().String("language", "", "Audio language (ISO 639-1 code, e.g. en, fr)")
	mediaTranscribeCmd.Flags().StringP("output", "o", "", "Write transcription to file instead of stdout")

	mediaSpeakCmd.Flags().String("voice", "", "Voice ID")
	mediaSpeakCmd.Flags().String("model", "", "TTS model ID")
	mediaSpeakCmd.Flags().Float64("speed", 0, "Speech speed")
	mediaSpeakCmd.Flags().Int("pitch", 0, "Speech pitch")
	mediaSpeakCmd.Flags().String("emotion", "", "Speech emotion")
	mediaSpeakCmd.Flags().StringP("output", "o", "", "Save audio to local file")
	mediaSpeakCmd.Flags().String("project", "", "Project ID")

	mediaListVoicesCmd.Flags().String("language", "", "Filter by language")
	mediaListVoicesCmd.Flags().String("gender", "", "Filter by gender")

	mediaCmd.AddCommand(mediaGenerateCmd)
	mediaCmd.AddCommand(mediaListModelsCmd)
	mediaCmd.AddCommand(mediaTranscribeCmd)
	mediaCmd.AddCommand(mediaSpeakCmd)
	mediaCmd.AddCommand(mediaListVoicesCmd)
	mediaCmd.AddCommand(mediaListTTSModelsCmd)
}
