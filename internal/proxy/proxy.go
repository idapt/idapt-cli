package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/idapt/idapt-cli/internal/listener"
)

type Proxy struct {
	defaultPort int
	transport   http.RoundTripper
}

func New(defaultPort int) *Proxy {
	return &Proxy{
		defaultPort: defaultPort,
		transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetPort := p.resolvePort(r)
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", targetPort))

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = r.Host // preserve original host
			pr.SetXForwarded()
		},
		Transport: p.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error for %s: %v", r.URL.Path, err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, badGatewayHTML, r.Host)
		},
	}

	proxy.ServeHTTP(w, r)
}

func (p *Proxy) resolvePort(r *http.Request) int {
	if port := listener.ListenerPortFromContext(r.Context()); port > 0 {
		return port
	}
	return p.defaultPort
}

var badGatewayHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>502 Bad Gateway</title>
<style>
body{font-family:system-ui,sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#111;color:#eee}
.c{text-align:center;max-width:500px;padding:2rem}
h1{font-size:2rem;margin-bottom:1rem}
p{color:#999;line-height:1.6}
a{color:#60a5fa;text-decoration:none}
a:hover{text-decoration:underline}
</style>
</head>
<body>
<div class="c">
<h1>502 Bad Gateway</h1>
<p>The service on <strong>%s</strong> is not responding. It may still be starting up or has crashed.</p>
<p><a href="https://idapt.ai">Go to idapt</a></p>
</div>
</body>
</html>`
