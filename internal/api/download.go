package api

import (
	"context"
	"io"
	"mime"
	"strconv"
)

type DownloadResult struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	Filename      string
}

func (c *Client) Download(ctx context.Context, path string) (*DownloadResult, error) {
	resp, err := c.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	result := &DownloadResult{
		Body:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
	}

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		result.ContentLength, _ = strconv.ParseInt(cl, 10, 64)
	}

	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			result.Filename = params["filename"]
		}
	}

	return result, nil
}
