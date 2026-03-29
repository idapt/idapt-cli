// Package httpclient provides a shared HTTP client that injects
// User-Agent and X-Idapt-Version headers on every request.
// See API_Versioning.md for the versioning strategy.
package httpclient

import (
	"net/http"
	"time"
)

// APIVersion is the API version this CLI was built for.
// Set at build time via -ldflags. Matches CURRENT_API_VERSION in TypeScript.
var APIVersion = "2026-03-28"

// versionTransport wraps an http.RoundTripper to inject version headers.
type versionTransport struct {
	base       http.RoundTripper
	userAgent  string
	apiVersion string
}

// RoundTrip implements http.RoundTripper. It clones the request and sets
// User-Agent and X-Idapt-Version headers before delegating to the base transport.
func (t *versionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("User-Agent", t.userAgent)
	if r.Header.Get("X-Idapt-Version") == "" {
		r.Header.Set("X-Idapt-Version", t.apiVersion)
	}
	return t.base.RoundTrip(r)
}

// New creates an HTTP client that injects User-Agent and X-Idapt-Version
// headers on every request.
func New(cliVersion string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &versionTransport{
			base:       http.DefaultTransport,
			userAgent:  "idapt-cli/" + cliVersion,
			apiVersion: APIVersion,
		},
	}
}
