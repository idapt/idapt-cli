package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/idapt/idapt-cli/internal/httpclient"
)

type ClientConfig struct {
	BaseURL    string
	APIKey     string
	Verbose    bool
	CLIVersion string
}

type Client struct {
	baseURL *url.URL
	apiKey  string
	verbose bool
	http    *http.Client
	errOut  io.Writer
}

func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://idapt.ai"
	}
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if cfg.CLIVersion == "" {
		cfg.CLIVersion = "dev"
	}
	return &Client{
		baseURL: base,
		apiKey:  cfg.APIKey,
		verbose: cfg.Verbose,
		http:    httpclient.New(cfg.CLIVersion, 60*time.Second),
		errOut:  io.Discard,
	}, nil
}

func (c *Client) APIKey() string {
	return c.apiKey
}

func (c *Client) SetErrOut(w io.Writer) {
	c.errOut = w
}

type RequestOption func(*http.Request)

func WithQuery(params url.Values) RequestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		for k, vs := range params {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
	}
}

func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	for _, opt := range opts {
		opt(req)
	}

	if c.verbose {
		fmt.Fprintf(c.errOut, "> %s %s\n", method, req.URL.String())
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseErrorResponse(resp)
	}

	return resp, nil
}

func (c *Client) DoJSON(ctx context.Context, method, path string, reqBody interface{}, respTarget interface{}) error {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	opts := []RequestOption{}
	if body != nil {
		opts = append(opts, WithHeader("Content-Type", "application/json"))
	}

	resp, err := c.Do(ctx, method, path, body, opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if respTarget == nil || resp.StatusCode == 204 {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(respTarget)
}

func (c *Client) Get(ctx context.Context, path string, query url.Values, respTarget interface{}) error {
	opts := []RequestOption{}
	if query != nil {
		opts = append(opts, WithQuery(query))
	}
	resp, err := c.Do(ctx, "GET", path, nil, opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if respTarget == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(respTarget)
}

func (c *Client) Post(ctx context.Context, path string, body interface{}, resp interface{}) error {
	return c.DoJSON(ctx, "POST", path, body, resp)
}

func (c *Client) Patch(ctx context.Context, path string, body interface{}, resp interface{}) error {
	return c.DoJSON(ctx, "PATCH", path, body, resp)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.DoJSON(ctx, "DELETE", path, nil, nil)
}
