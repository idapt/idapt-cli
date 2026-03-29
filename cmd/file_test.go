package cmd

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("projectId") == "" {
					t.Error("expected projectId query param")
				}
				jsonResponse(w, 200, map[string]interface{}{
					"files": []map[string]interface{}{
						{"id": "f1", "name": "readme.md", "type": "file", "size": 1024, "updatedAt": "2025-01-01"},
						{"id": "f2", "name": "src", "type": "folder", "size": 0, "updatedAt": "2025-01-02"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "file", "list", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 2 {
			t.Fatalf("expected 2 files, got %d", len(items))
		}
		if items[0]["name"] != "readme.md" {
			t.Errorf("expected readme.md, got: %v", items[0]["name"])
		}
	})

	t.Run("with path argument", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("path") != "/docs" {
					t.Errorf("expected path /docs, got: %v", r.URL.Query().Get("path"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"files": []map[string]interface{}{},
				})
			},
		})
		_, _, err := runCmd(t, handler, "file", "list", "/docs", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"files": []map[string]interface{}{},
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "file", "list", "--project", testProjectID)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 0 {
			t.Errorf("expected empty list, got %d items", len(items))
		}
	})
}

func TestFileRead(t *testing.T) {
	t.Run("success streams content", func(t *testing.T) {
		fileContent := "Hello, world!\nLine two."
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files/file-123/download": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(fileContent))
			},
		})
		stdout, _, err := runCmd(t, handler, "file", "read", "file-123")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if stdout != fileContent {
			t.Errorf("expected file content, got: %q", stdout)
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files/bad-id/download": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 404, "File not found")
			},
		})
		_, _, err := runCmd(t, handler, "file", "read", "bad-id")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})
}

func TestFileWrite(t *testing.T) {
	t.Run("writes stdin content", func(t *testing.T) {
		var receivedContent string
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/files/file-w1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				receivedContent, _ = body["content"].(string)
				w.WriteHeader(200)
				jsonResponse(w, 200, map[string]interface{}{})
			},
		})

		env := newTestEnv(t, handler)
		env.factory.In = strings.NewReader("Written via stdin")

		stdout, _, err := runCmdWithEnv(t, env, "file", "write", "file-w1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if receivedContent != "Written via stdin" {
			t.Errorf("expected stdin content, got: %q", receivedContent)
		}
		if !strings.Contains(stdout, "File updated") {
			t.Errorf("expected update message, got: %s", stdout)
		}
	})
}

func TestFileCreate(t *testing.T) {
	t.Run("with content flag", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/files": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "notes.txt" {
					t.Errorf("expected name notes.txt, got: %v", body["name"])
				}
				if body["content"] != "Initial content" {
					t.Errorf("expected content, got: %v", body["content"])
				}
				if body["projectId"] == nil || body["projectId"] == "" {
					t.Error("expected projectId")
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "new-f", "name": "notes.txt",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"file", "create", "notes.txt",
			"--project", testProjectID,
			"--content", "Initial content",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["id"] != "new-f" {
			t.Errorf("expected id new-f, got: %v", parsed["id"])
		}
	})

	t.Run("with parent", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/files": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["parentId"] != "folder-123" {
					t.Errorf("expected parentId folder-123, got: %v", body["parentId"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "child-f", "name": "child.md",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"file", "create", "child.md",
			"--project", testProjectID,
			"--parent", "folder-123",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestFileEdit(t *testing.T) {
	t.Run("find and replace success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files/fe-1/download": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("Hello world, this is a test."))
			},
			"PATCH /api/files/fe-1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				content, _ := body["content"].(string)
				if !strings.Contains(content, "universe") {
					t.Errorf("expected 'world' replaced with 'universe', got: %s", content)
				}
				if strings.Contains(content, "world") {
					t.Errorf("expected 'world' to be replaced, but it still exists")
				}
				w.WriteHeader(200)
				jsonResponse(w, 200, map[string]interface{}{})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"file", "edit", "fe-1",
			"--old-text", "world",
			"--new-text", "universe",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "File updated") {
			t.Errorf("expected update message, got: %s", stdout)
		}
	})

	t.Run("pattern not found", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files/fe-2/download": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("Some content here."))
			},
		})
		_, _, err := runCmd(t, handler,
			"file", "edit", "fe-2",
			"--old-text", "nonexistent pattern",
			"--new-text", "replacement",
		)
		if err == nil {
			t.Fatal("expected error when pattern not found")
		}
		if !strings.Contains(err.Error(), "old-text not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("missing old-text flag", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "file", "edit", "fe-3", "--new-text", "something")
		if err == nil {
			t.Fatal("expected error for missing --old-text")
		}
		if !strings.Contains(err.Error(), "--old-text is required") {
			t.Errorf("expected required flag error, got: %v", err)
		}
	})
}

func TestFileDelete(t *testing.T) {
	t.Run("with confirm", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"DELETE /api/files/fd-1": func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(204)
			},
		})
		stdout, _, err := runCmdConfirm(t, handler, "file", "delete", "fd-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "File deleted") {
			t.Errorf("expected delete message, got: %s", stdout)
		}
	})

	t.Run("without confirm aborts", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "file", "delete", "fd-2")
		if err == nil {
			t.Fatal("expected abort error")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected aborted, got: %v", err)
		}
	})
}

func TestFileRename(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/files/fr-1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "new-name.txt" {
					t.Errorf("expected name new-name.txt, got: %v", body["name"])
				}
				w.WriteHeader(200)
				jsonResponse(w, 200, map[string]interface{}{})
			},
		})
		stdout, _, err := runCmd(t, handler, "file", "rename", "fr-1", "new-name.txt")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Renamed to new-name.txt") {
			t.Errorf("expected rename message, got: %s", stdout)
		}
	})
}

func TestFileMkdir(t *testing.T) {
	t.Run("creates folder", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/files": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["name"] != "docs" {
					t.Errorf("expected name docs, got: %v", body["name"])
				}
				if body["type"] != "folder" {
					t.Errorf("expected type folder, got: %v", body["type"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "dir-1", "name": "docs",
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"file", "mkdir", "docs",
			"--project", testProjectID,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["name"] != "docs" {
			t.Errorf("expected name docs, got: %v", parsed["name"])
		}
	})

	t.Run("with parent", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/files": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["parentId"] != "parent-folder" {
					t.Errorf("expected parentId, got: %v", body["parentId"])
				}
				jsonResponse(w, 201, map[string]interface{}{
					"id": "dir-2", "name": "subdir",
				})
			},
		})
		_, _, err := runCmd(t, handler,
			"file", "mkdir", "subdir",
			"--project", testProjectID,
			"--parent", "parent-folder",
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestFileGrep(t *testing.T) {
	t.Run("returns matching results", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/search": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("q") != "TODO" {
					t.Errorf("expected query TODO, got: %v", r.URL.Query().Get("q"))
				}
				if r.URL.Query().Get("type") != "content" {
					t.Errorf("expected type content, got: %v", r.URL.Query().Get("type"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"results": []map[string]interface{}{
						{"name": "main.go", "snippet": "// TODO: fix this"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"file", "grep", "TODO",
			"--project", testProjectID,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 1 {
			t.Fatalf("expected 1 result, got %d", len(items))
		}
	})
}

func TestFileUpload(t *testing.T) {
	t.Run("uploads file with multipart", func(t *testing.T) {
		var receivedContentType string
		var receivedBody []byte
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/files": func(w http.ResponseWriter, r *http.Request) {
				receivedContentType = r.Header.Get("Content-Type")
				receivedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(201)
			},
		})

		// Create a temporary file to upload
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test-upload.txt")
		if err := os.WriteFile(tmpFile, []byte("upload content"), 0644); err != nil {
			t.Fatalf("creating temp file: %v", err)
		}

		stdout, _, err := runCmd(t, handler,
			"file", "upload", tmpFile,
			"--project", testProjectID,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(receivedContentType, "multipart/form-data") {
			t.Errorf("expected multipart content type, got: %s", receivedContentType)
		}
		if len(receivedBody) == 0 {
			t.Error("expected non-empty body")
		}
		if !strings.Contains(stdout, "Uploaded") {
			t.Errorf("expected upload message, got: %s", stdout)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler,
			"file", "upload", "/nonexistent/path/file.txt",
			"--project", testProjectID,
		)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "opening file") {
			t.Errorf("expected open error, got: %v", err)
		}
	})
}

func TestFileDownload(t *testing.T) {
	t.Run("downloads to specified path", func(t *testing.T) {
		binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/files/dl-1/download": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.Header().Set("Content-Disposition", `attachment; filename="image.png"`)
				w.WriteHeader(200)
				_, _ = w.Write(binaryContent)
			},
		})

		tmpDir := t.TempDir()
		outPath := filepath.Join(tmpDir, "downloaded.png")

		stdout, _, err := runCmd(t, handler, "file", "download", "dl-1", outPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify file was written
		data, readErr := os.ReadFile(outPath)
		if readErr != nil {
			t.Fatalf("reading downloaded file: %v", readErr)
		}
		if len(data) != len(binaryContent) {
			t.Errorf("expected %d bytes, got %d", len(binaryContent), len(data))
		}
		if !strings.Contains(stdout, "Downloaded") {
			t.Errorf("expected download message, got: %s", stdout)
		}
	})
}

func TestFileMove(t *testing.T) {
	t.Run("moves file to target parent", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"PATCH /api/files/fm-1": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["parentId"] != "target-folder" {
					t.Errorf("expected parentId target-folder, got: %v", body["parentId"])
				}
				w.WriteHeader(200)
				jsonResponse(w, 200, map[string]interface{}{})
			},
		})
		stdout, _, err := runCmd(t, handler, "file", "move", "fm-1", "target-folder")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "File moved") {
			t.Errorf("expected move message, got: %s", stdout)
		}
	})
}

func TestFileSearch(t *testing.T) {
	t.Run("searches by name", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/search": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("q") != "readme" {
					t.Errorf("expected query readme, got: %v", r.URL.Query().Get("q"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"results": []map[string]interface{}{
						{"id": "s1", "name": "README.md", "type": "file", "path": "/README.md"},
					},
				})
			},
		})
		stdout, _, err := runCmd(t, handler,
			"file", "search", "readme",
			"--project", testProjectID,
		)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		items := parseJSONArrayOutput(t, stdout)
		if len(items) != 1 {
			t.Fatalf("expected 1 result, got %d", len(items))
		}
		if items[0]["name"] != "README.md" {
			t.Errorf("expected README.md, got: %v", items[0]["name"])
		}
	})
}
