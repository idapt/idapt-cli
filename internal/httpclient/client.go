package httpclient

import (
	"net/http"
	"time"
)

var APIVersion = "2026-03-28"

type versionTransport struct {
	base       http.RoundTripper
	userAgent  string
	apiVersion string
}

func (t *versionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("User-Agent", t.userAgent)
	if r.Header.Get("X-Idapt-Version") == "" {
		r.Header.Set("X-Idapt-Version", t.apiVersion)
	}
	return t.base.RoundTrip(r)
}

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
