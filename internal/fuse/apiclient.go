package fuse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/idapt/idapt-cli/internal/api"
)

type FuseAPIClient struct {
	client  *api.Client
	timeout time.Duration
}

func NewFuseAPIClient(client *api.Client) *FuseAPIClient {
	return &FuseAPIClient{
		client:  client,
		timeout: 30 * time.Second,
	}
}

type FileEntry struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ParentID         *string   `json:"parentId"`
	ProjectID        string    `json:"projectId"`
	BlobID           *string   `json:"blobId"`
	Size             int64     `json:"fileSize"`
	MimeType         string    `json:"mimeType"`
	IsFolder         bool      `json:"-"` // derived: blobId == nil
	Version          int       `json:"version"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	ResourceID       string    `json:"resourceId"`
	Icon             string    `json:"icon"`
	Prompt           string    `json:"prompt"`
	DurationMs       int       `json:"durationMs"`
	PublicAccess     string    `json:"publicAccess"`
	IsSensitive      bool      `json:"isSensitive"`
	Extension        string    `json:"extension"`
	CreatedByActorID string    `json:"createdByActorId"`
}

func (c *FuseAPIClient) ListFiles(ctx context.Context, projectID, folderID string) ([]FileEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute) // longer timeout for pagination
	defer cancel()

	const pageSize = "1000"
	var allFiles []FileEntry
	startingAfter := ""

	for {
		query := url.Values{"projectId": {projectID}, "limit": {pageSize}}
		if folderID != "" {
			query.Set("folderId", folderID)
		}
		if startingAfter != "" {
			query.Set("startingAfter", startingAfter)
		}

		var resp struct {
			Files   []FileEntry `json:"files"`
			HasMore bool        `json:"hasMore"`
		}
		if err := c.client.Get(ctx, "/api/files/list", query, &resp); err != nil {
			return allFiles, mapAPIError(err)
		}

		for i := range resp.Files {
			resp.Files[i].IsFolder = resp.Files[i].BlobID == nil
		}
		allFiles = append(allFiles, resp.Files...)

		if !resp.HasMore || len(resp.Files) == 0 {
			break
		}
		startingAfter = resp.Files[len(resp.Files)-1].ID
	}

	return allFiles, nil
}

func (c *FuseAPIClient) GetFileMetadata(ctx context.Context, fileID string) (*FileEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var entry FileEntry
	if err := c.client.Get(ctx, "/api/files/"+fileID+"/metadata", nil, &entry); err != nil {
		return nil, mapAPIError(err)
	}
	entry.IsFolder = entry.BlobID == nil
	return &entry, nil
}

func (c *FuseAPIClient) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

	resp, err := c.client.Do(dlCtx, "GET", "/api/files/"+fileID+"/download", nil)
	if err != nil {
		cancel()
		return nil, mapAPIError(err)
	}
	return &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func (c *FuseAPIClient) GetFileVersion(ctx context.Context, fileID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp struct {
		Version int `json:"version"`
	}
	if err := c.client.Get(ctx, "/api/files/"+fileID+"/version", nil, &resp); err != nil {
		return 0, mapAPIError(err)
	}
	return resp.Version, nil
}

func (c *FuseAPIClient) GetFileVersionsBatch(ctx context.Context, fileIDs []string) (map[string]int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp map[string]int
	if err := c.client.Post(ctx, "/api/files/versions", map[string]interface{}{
		"fileIds": fileIDs,
	}, &resp); err != nil {
		return nil, mapAPIError(err)
	}
	return resp, nil
}

func (c *FuseAPIClient) CreateFile(ctx context.Context, projectID, parentID, name string, content []byte, mimeType string) (*FileEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	body, contentType, err := buildMultipartForm(projectID, parentID, name, content, mimeType)
	if err != nil {
		return nil, fmt.Errorf("build form: %w", err)
	}

	resp, err := c.client.Do(ctx, "POST", "/api/files", body,
		api.WithHeader("Content-Type", contentType))
	if err != nil {
		return nil, mapAPIError(err)
	}
	defer resp.Body.Close()

	var result struct {
		FileID string `json:"fileId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.GetFileMetadata(ctx, result.FileID)
}

func (c *FuseAPIClient) CreateFolder(ctx context.Context, projectID, parentID, name string) (*FileEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body := map[string]interface{}{
		"name":      name,
		"projectId": projectID,
	}
	if parentID != "" {
		body["parentId"] = parentID
	}

	var resp struct {
		Folder FileEntry `json:"folder"`
	}
	if err := c.client.Post(ctx, "/api/files/folders", body, &resp); err != nil {
		return nil, mapAPIError(err)
	}
	resp.Folder.IsFolder = true
	return &resp.Folder, nil
}

func (c *FuseAPIClient) UpdateFileContent(ctx context.Context, fileID string, content string, expectedVersion int) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	err := c.client.Patch(ctx, "/api/files/"+fileID, map[string]interface{}{
		"content":         content,
		"expectedVersion": expectedVersion,
	}, nil)
	return mapAPIError(err)
}

func (c *FuseAPIClient) RenameFile(ctx context.Context, fileID, newName string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Patch(ctx, "/api/files/"+fileID, map[string]interface{}{
		"name": newName,
	}, nil)
}

func (c *FuseAPIClient) MoveFile(ctx context.Context, fileID, newParentID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Post(ctx, "/api/files/"+fileID+"/move", map[string]interface{}{
		"parentId": newParentID,
	}, nil)
}

func (c *FuseAPIClient) TrashFile(ctx context.Context, fileID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Delete(ctx, "/api/files/"+fileID)
}

func (c *FuseAPIClient) UploadLargeFile(ctx context.Context, fileID string, content []byte, mimeType string, expectedVersion int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var uploadResp struct {
		URL    string            `json:"url"`
		Fields map[string]string `json:"fields"`
	}
	if err := c.client.Post(ctx, "/api/files/upload-url", map[string]interface{}{
		"fileId":      fileID,
		"fileSize":    len(content),
		"contentType": mimeType,
	}, &uploadResp); err != nil {
		return fmt.Errorf("get upload URL: %w", mapAPIError(err))
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadResp.URL, io.NopCloser(bytesReader(content)))
	if err != nil {
		return fmt.Errorf("create S3 request: %w", err)
	}
	req.Header.Set("Content-Type", mimeType)
	req.ContentLength = int64(len(content))

	s3Resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("S3 upload: %w", err)
	}
	defer s3Resp.Body.Close()

	if s3Resp.StatusCode >= 400 {
		return fmt.Errorf("S3 upload failed: %d", s3Resp.StatusCode)
	}

	err = c.client.Post(ctx, "/api/files/finalize", map[string]interface{}{
		"fileId":          fileID,
		"expectedVersion": expectedVersion,
	}, nil)
	return mapAPIError(err)
}

const LargeFileThreshold = 1024 * 1024 // 1MB

func mapAPIError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *api.APIError
	if ok := isAPIError(err, &apiErr); ok {
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			return syscall.ENOENT
		case http.StatusForbidden:
			return syscall.EACCES
		case http.StatusConflict:
			return syscall.ESTALE
		case http.StatusTooManyRequests:
			return syscall.EAGAIN
		case http.StatusUnauthorized:
			return syscall.EACCES
		default:
			return syscall.EIO
		}
	}

	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") {
		return syscall.EIO
	}

	return syscall.EIO
}

func isAPIError(err error, target **api.APIError) bool {
	if apiErr, ok := err.(*api.APIError); ok {
		*target = apiErr
		return true
	}
	return false
}
