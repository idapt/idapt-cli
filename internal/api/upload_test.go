package api

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestUpload(t *testing.T) {
	t.Run("multipart form with file and fields", func(t *testing.T) {
		var (
			gotFilename    string
			gotFileContent string
			gotFieldA      string
			gotFieldB      string
			gotBoundary    bool
		)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			mediaType, params, err := mime.ParseMediaType(ct)
			if err != nil {
				t.Errorf("parse Content-Type: %v", err)
				w.WriteHeader(400)
				return
			}
			if mediaType != "multipart/form-data" {
				t.Errorf("media type = %q, want multipart/form-data", mediaType)
			}
			if params["boundary"] != "" {
				gotBoundary = true
			}

			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("NextPart: %v", err)
					break
				}
				data, _ := io.ReadAll(part)
				if part.FormName() == "file" {
					gotFilename = part.FileName()
					gotFileContent = string(data)
				} else if part.FormName() == "fieldA" {
					gotFieldA = string(data)
				} else if part.FormName() == "fieldB" {
					gotFieldB = string(data)
				}
			}
			w.WriteHeader(200)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		fields := map[string]string{"fieldA": "valueA", "fieldB": "valueB"}
		resp, err := c.Upload(context.Background(), "/upload", "test.txt", strings.NewReader("file content here"), fields)
		if err != nil {
			t.Fatalf("Upload error: %v", err)
		}
		resp.Body.Close()

		if !gotBoundary {
			t.Fatal("Content-Type missing boundary parameter")
		}
		if gotFilename != "test.txt" {
			t.Fatalf("filename = %q, want %q", gotFilename, "test.txt")
		}
		if gotFileContent != "file content here" {
			t.Fatalf("file content = %q, want %q", gotFileContent, "file content here")
		}
		if gotFieldA != "valueA" {
			t.Fatalf("fieldA = %q, want %q", gotFieldA, "valueA")
		}
		if gotFieldB != "valueB" {
			t.Fatalf("fieldB = %q, want %q", gotFieldB, "valueB")
		}
	})

	t.Run("empty fields map", func(t *testing.T) {
		var partCount int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				_, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("NextPart: %v", err)
					break
				}
				partCount++
			}
			w.WriteHeader(200)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		resp, err := c.Upload(context.Background(), "/upload", "data.bin", strings.NewReader("binary"), nil)
		if err != nil {
			t.Fatalf("Upload error: %v", err)
		}
		resp.Body.Close()

		// Only the file part should be present
		if partCount != 1 {
			t.Fatalf("part count = %d, want 1 (file only)", partCount)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(413)
			_, _ = w.Write([]byte(`{"error":"file too large"}`))
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		_, err := c.Upload(context.Background(), "/upload", "big.zip", strings.NewReader("data"), nil)
		if err == nil {
			t.Fatal("expected error for 413 response")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("error type = %T, want *APIError", err)
		}
		if apiErr.StatusCode != 413 {
			t.Fatalf("StatusCode = %d, want 413", apiErr.StatusCode)
		}
	})
}

func TestDownload(t *testing.T) {
	t.Run("successful download with headers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Length", "12")
			w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
			_, _ = w.Write([]byte("pdf-contents"))
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		result, err := c.Download(context.Background(), "/files/123/download")
		if err != nil {
			t.Fatalf("Download error: %v", err)
		}
		defer result.Body.Close()

		if result.ContentType != "application/pdf" {
			t.Fatalf("ContentType = %q, want %q", result.ContentType, "application/pdf")
		}
		if result.Filename != "report.pdf" {
			t.Fatalf("Filename = %q, want %q", result.Filename, "report.pdf")
		}
		if result.ContentLength != 12 {
			t.Fatalf("ContentLength = %d, want 12", result.ContentLength)
		}
		body, _ := io.ReadAll(result.Body)
		if string(body) != "pdf-contents" {
			t.Fatalf("body = %q, want %q", string(body), "pdf-contents")
		}
	})

	t.Run("missing Content-Disposition", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello"))
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		result, err := c.Download(context.Background(), "/files/456/download")
		if err != nil {
			t.Fatalf("Download error: %v", err)
		}
		defer result.Body.Close()

		if result.Filename != "" {
			t.Fatalf("Filename = %q, want empty when Content-Disposition missing", result.Filename)
		}
		if result.ContentType != "text/plain" {
			t.Fatalf("ContentType = %q, want %q", result.ContentType, "text/plain")
		}
	})

	t.Run("server error returns APIError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		c := &Client{
			baseURL: u,
			apiKey:  "key",
			http:    &http.Client{Timeout: 5 * time.Second},
			errOut:  io.Discard,
		}

		_, err := c.Download(context.Background(), "/files/789/download")
		if err == nil {
			t.Fatal("expected error for 403 response")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("error type = %T, want *APIError", err)
		}
		if apiErr.StatusCode != 403 {
			t.Fatalf("StatusCode = %d, want 403", apiErr.StatusCode)
		}
	})
}
