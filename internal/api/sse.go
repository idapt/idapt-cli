package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// SSEReader reads SSE events from a response stream.
type SSEReader struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
	err     error
}

// StreamSSE opens an SSE connection and returns a reader.
func (c *Client) StreamSSE(ctx context.Context, method, path string, reqBody interface{}) (*SSEReader, error) {
	var body io.Reader
	opts := []RequestOption{
		WithHeader("Accept", "text/event-stream"),
	}

	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
		opts = append(opts, WithHeader("Content-Type", "application/json"))
	}

	resp, err := c.Do(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
	}

	return newSSEReader(resp.Body), nil
}

func newSSEReader(body io.ReadCloser) *SSEReader {
	return &SSEReader{
		scanner: bufio.NewScanner(body),
		body:    body,
	}
}

// NewSSEReaderFromReader creates an SSEReader from an io.ReadCloser (for testing).
func NewSSEReaderFromReader(r io.ReadCloser) *SSEReader {
	return newSSEReader(r)
}

// Next reads the next SSE event. Returns nil and io.EOF when the stream ends.
func (r *SSEReader) Next() (*SSEEvent, error) {
	if r.err != nil {
		return nil, r.err
	}

	event := &SSEEvent{}
	hasData := false

	for r.scanner.Scan() {
		line := r.scanner.Text()

		if line == "" {
			if hasData || event.Event != "" || event.ID != "" {
				return event, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			event.Event = value
		case "data":
			if hasData {
				event.Data += "\n" + value
			} else {
				event.Data = value
				hasData = true
			}
		case "id":
			event.ID = value
		}
	}

	if err := r.scanner.Err(); err != nil {
		r.err = err
		return nil, err
	}

	// Stream ended - if we have partial data, return it
	if hasData || event.Event != "" || event.ID != "" {
		r.err = io.EOF
		return event, nil
	}

	r.err = io.EOF
	return nil, io.EOF
}

// Close closes the underlying response body.
func (r *SSEReader) Close() error {
	return r.body.Close()
}

// StreamSSEGet opens a GET SSE connection.
func (c *Client) StreamSSEGet(ctx context.Context, path string, query ...RequestOption) (*SSEReader, error) {
	opts := append([]RequestOption{
		WithHeader("Accept", "text/event-stream"),
	}, query...)

	resp, err := c.Do(ctx, "GET", path, nil, opts...)
	if err != nil {
		return nil, err
	}

	return newSSEReader(resp.Body), nil
}
