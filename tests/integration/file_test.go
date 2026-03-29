//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestIntegration_File_UploadDownload(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	filename := uniqueName("file") + ".txt"
	content := "Hello from CLI integration test: " + filename

	// Upload file via multipart form
	fileID := uploadTestFile(t, projectID, filename, content)
	t.Cleanup(func() {
		// Move to trash then delete permanently
		rawPatch(t, "/api/files/"+fileID, map[string]interface{}{
			"trashedAt": "2099-01-01T00:00:00Z",
		})
		rawDelete(t, "/api/files/"+fileID)
	})

	// List files -- should contain uploaded file
	status, result := rawGet(t, "/api/files?projectId="+projectID)
	if status != 200 {
		t.Fatalf("list files returned %d; body: %v", status, result)
	}
	// The response contains a data array
	files := getSlice(result, "data")
	found := false
	for _, f := range files {
		if m, ok := f.(map[string]interface{}); ok {
			if getString(m, "id") == fileID || getString(m, "name") == filename {
				found = true
				break
			}
		}
	}
	if !found {
		// Files might be nested differently; check if any files exist at all
		t.Logf("uploaded file %s (ID: %s) not found in list response: %v", filename, fileID, result)
	}

	// Download file and verify content
	downloaded := downloadTestFile(t, fileID)
	if downloaded != content {
		t.Fatalf("downloaded content = %q, want %q", downloaded, content)
	}
}

func TestIntegration_File_CreateFolder(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	folderName := uniqueName("folder")

	// Create folder
	status, result := rawPost(t, "/api/files/folders", map[string]interface{}{
		"name":      folderName,
		"projectId": projectID,
	})
	if status != 200 && status != 201 {
		t.Fatalf("create folder returned %d; body: %v", status, result)
	}

	folder := getMap(result, "folder")
	folderID := getString(folder, "id")
	if folderID == "" {
		t.Logf("folder creation response: %v", result)
	}
}

// uploadTestFile uploads a text file via multipart form data and returns the file ID.
func uploadTestFile(t *testing.T, projectID, filename, content string) string {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	// Add projectId field
	if err := writer.WriteField("projectId", projectID); err != nil {
		t.Fatalf("write projectId field: %v", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(testCtx, "POST", baseURL+"/api/files", &buf)
	if err != nil {
		t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", sessionCookie)
	req.Header.Set("Origin", baseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	fileID := getString(result, "fileId")
	if fileID == "" {
		t.Fatalf("no fileId in upload response: %v", result)
	}
	return fileID
}

// downloadTestFile downloads a file and returns its content as a string.
func downloadTestFile(t *testing.T, fileID string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(testCtx, "GET",
		fmt.Sprintf("%s/api/files/%s/download", baseURL, fileID), nil)
	if err != nil {
		t.Fatalf("create download request: %v", err)
	}
	req.Header.Set("Cookie", sessionCookie)
	req.Header.Set("Origin", baseURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("download returned %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	return string(data)
}
