package fuse

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
)

func buildMultipartForm(projectID, parentID, name string, content []byte, mimeType string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("projectId", projectID); err != nil {
		return nil, "", fmt.Errorf("write projectId: %w", err)
	}

	if parentID != "" {
		if err := w.WriteField("parentId", parentID); err != nil {
			return nil, "", fmt.Errorf("write parentId: %w", err)
		}
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	h.Set("Content-Type", mimeType)

	part, err := w.CreatePart(h)
	if err != nil {
		return nil, "", fmt.Errorf("create file part: %w", err)
	}

	if _, err := part.Write(content); err != nil {
		return nil, "", fmt.Errorf("write file content: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("close writer: %w", err)
	}

	return &buf, w.FormDataContentType(), nil
}
