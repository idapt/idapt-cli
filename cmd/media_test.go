package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected inputType
	}{
		{
			name:     "HTTP URL",
			input:    "http://example.com/photo.png",
			expected: inputTypeURL,
		},
		{
			name:     "HTTPS URL",
			input:    "https://example.com/images/photo.png",
			expected: inputTypeURL,
		},
		{
			name:     "remote idapt path with 3+ slashes",
			input:    "/alice/personal/files/Photos/photo.png",
			expected: inputTypeRemotePath,
		},
		{
			name:     "remote idapt path minimal",
			input:    "/owner/project/files",
			expected: inputTypeRemotePath,
		},
		{
			name:     "local file relative",
			input:    "./photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "local file bare name",
			input:    "photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "local file with directory",
			input:    "images/photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "short absolute path is local (only 2 slashes)",
			input:    "/tmp/photo.png",
			expected: inputTypeLocalFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyInput(tt.input)
			if result != tt.expected {
				t.Errorf("classifyInput(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMediaSpeak(t *testing.T) {
	defaultProject := "00000000-0000-0000-0000-000000000001"

	t.Run("basic text", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/audio/generate": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["text"] != "Hello world" {
					t.Errorf("expected text='Hello world', got %v", body["text"])
				}
				if body["projectId"] != defaultProject {
					t.Errorf("expected projectId=%s, got %v", defaultProject, body["projectId"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"url": "https://cdn.example.com/audio/123.mp3",
				})
			},
		})
		stdout, _, err := runCmd(t, h, "media", "speak", "Hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout, "https://cdn.example.com/audio/123.mp3") {
			t.Errorf("expected URL in output, got: %s", stdout)
		}
	})

	t.Run("with voice and model flags", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/audio/generate": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["voiceId"] != "alloy" {
					t.Errorf("expected voiceId=alloy, got %v", body["voiceId"])
				}
				if body["modelId"] != "tts-1-hd" {
					t.Errorf("expected modelId=tts-1-hd, got %v", body["modelId"])
				}
				if body["text"] != "Test speech" {
					t.Errorf("expected text='Test speech', got %v", body["text"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"url": "https://cdn.example.com/audio/456.mp3",
				})
			},
		})
		stdout, _, err := runCmd(t, h, "media", "speak", "--voice", "alloy", "--model", "tts-1-hd", "Test speech")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout, "https://cdn.example.com/audio/456.mp3") {
			t.Errorf("expected URL in output, got: %s", stdout)
		}
	})

	t.Run("reads from stdin", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/audio/generate": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["text"] != "stdin text content" {
					t.Errorf("expected text='stdin text content', got %v", body["text"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"url": "https://cdn.example.com/audio/stdin.mp3",
				})
			},
		})
		env := newTestEnv(t, h)
		env.factory.In = strings.NewReader("stdin text content")
		stdout, _, err := runCmdWithEnv(t, env, "media", "speak", "-")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout, "https://cdn.example.com/audio/stdin.mp3") {
			t.Errorf("expected URL in output, got: %s", stdout)
		}
	})

	t.Run("reads from file with @ prefix", func(t *testing.T) {
		// Create a temp file with text content
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "speech.txt")
		if err := os.WriteFile(tmpFile, []byte("file text content"), 0644); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}

		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/audio/generate": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["text"] != "file text content" {
					t.Errorf("expected text='file text content', got %v", body["text"])
				}
				jsonResponse(w, 200, map[string]interface{}{
					"url": "https://cdn.example.com/audio/file.mp3",
				})
			},
		})
		stdout, _, err := runCmd(t, h, "media", "speak", "@"+tmpFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout, "https://cdn.example.com/audio/file.mp3") {
			t.Errorf("expected URL in output, got: %s", stdout)
		}
	})

	t.Run("empty text error", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		env := newTestEnv(t, h)
		env.factory.In = strings.NewReader("")
		_, _, err := runCmdWithEnv(t, env, "media", "speak", "-")
		if err == nil {
			t.Fatal("expected error for empty text")
		}
		if !strings.Contains(err.Error(), "text is empty") {
			t.Errorf("expected 'text is empty' error, got: %v", err)
		}
	})

	t.Run("missing project error", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		env := newTestEnv(t, h)
		env.factory.Config.DefaultProject = "" // Clear default project
		_, _, err := runCmdWithEnv(t, env, "media", "speak", "Hello")
		if err == nil {
			t.Fatal("expected error for missing project")
		}
		if !strings.Contains(err.Error(), "--project flag or default project is required") {
			t.Errorf("expected project required error, got: %v", err)
		}
	})
}

func TestMediaListVoices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/audio/voices": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"voices": []map[string]interface{}{
						{"id": "alloy", "name": "Alloy", "gender": "neutral", "language": "en", "category": "standard"},
						{"id": "nova", "name": "Nova", "gender": "female", "language": "en", "category": "premium"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, h, "media", "list-voices", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got []map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(stdout), &got); jsonErr != nil {
			t.Fatalf("parsing JSON output: %v\nraw: %s", jsonErr, stdout)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 voices, got %d", len(got))
		}
	})

	t.Run("with language filter", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/audio/voices": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("language") != "fr" {
					t.Errorf("expected language=fr, got %v", r.URL.Query().Get("language"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"voices": []map[string]interface{}{
						{"id": "amelie", "name": "Amelie", "gender": "female", "language": "fr", "category": "standard"},
					},
				})
			},
		})
		_, _, err := runCmd(t, h, "media", "list-voices", "--language", "fr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMediaListTTSModels(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/audio/models": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"models": []map[string]interface{}{
						{"id": "tts-1", "name": "TTS-1", "costPer1kChars": 0.015, "speed": "fast"},
						{"id": "tts-1-hd", "name": "TTS-1 HD", "costPer1kChars": 0.030, "speed": "normal"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, h, "media", "list-tts-models", "-o", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got []map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(stdout), &got); jsonErr != nil {
			t.Fatalf("parsing JSON output: %v\nraw: %s", jsonErr, stdout)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 models, got %d", len(got))
		}
	})

	t.Run("rejects args", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, h, "media", "list-tts-models", "extra-arg")
		if err == nil {
			t.Fatal("expected error for extra argument")
		}
	})
}
