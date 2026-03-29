package api

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
)

// Upload sends a multipart/form-data request with a file and optional form fields.
func (c *Client) Upload(ctx context.Context, path string, filename string, reader io.Reader, fields map[string]string) (*http.Response, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		for k, v := range fields {
			_ = writer.WriteField(k, v)
		}
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, reader); err != nil {
			pw.CloseWithError(err)
			return
		}
		writer.Close()
	}()

	return c.Do(ctx, "POST", path, pr,
		WithHeader("Content-Type", writer.FormDataContentType()),
	)
}
