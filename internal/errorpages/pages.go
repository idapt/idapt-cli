package errorpages

import (
	"fmt"
	"html"
	"net/http"
)

// Pages serves error pages for various states.
type Pages struct {
	domain string
	appURL string
}

// New creates error page renderer.
func New(domain, appURL string) *Pages {
	return &Pages{domain: domain, appURL: appURL}
}

// ServeUnauthenticated serves the authentication required page.
func (p *Pages) ServeUnauthenticated(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, unauthenticatedHTML, html.EscapeString(p.domain), html.EscapeString(p.appURL))
}

// ServeBadGateway serves the 502 bad gateway page.
func (p *Pages) ServeBadGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, badGatewayHTML, html.EscapeString(p.domain), html.EscapeString(p.appURL))
}

// ServePortNotOpen serves the port not accessible page.
func (p *Pages) ServePortNotOpen(w http.ResponseWriter, r *http.Request, port int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, portNotOpenHTML, html.EscapeString(p.domain), port, html.EscapeString(p.appURL))
}

// ServeServiceUnavailable serves the 503 page.
func (p *Pages) ServeServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, serviceUnavailableHTML, html.EscapeString(p.domain), html.EscapeString(p.appURL))
}

// All pages are >512 bytes to prevent browser/CDN error page interception.
// All pages use html.EscapeString for slug/domain to prevent XSS.

var pageStyle = `
body{font-family:system-ui,-apple-system,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#0a0a0a;color:#e5e5e5}
@media(prefers-color-scheme:light){body{background:#fafafa;color:#171717}}
.c{text-align:center;max-width:500px;padding:2rem}
h1{font-size:1.75rem;margin-bottom:1rem;font-weight:600}
p{color:#a3a3a3;line-height:1.6;margin:0.5rem 0}
@media(prefers-color-scheme:light){p{color:#525252}}
a{color:#60a5fa;text-decoration:none}
a:hover{text-decoration:underline}
.btn{display:inline-block;padding:0.5rem 1.5rem;background:#2563eb;color:#fff;border-radius:0.375rem;margin-top:1rem;font-weight:500}
.btn:hover{background:#1d4ed8;text-decoration:none}
`

var unauthenticatedHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Authentication Required</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="c">
<h1>Authentication Required</h1>
<p>You need to sign in to access <strong>%s</strong>.</p>
<p>Please log in through the idapt app to get access to this machine.</p>
<a class="btn" href="%s">Sign in at idapt</a>
<p style="margin-top:2rem;font-size:0.85rem">If you have an API key, include it as <code>Authorization: Bearer mk_...</code></p>
</div>
</body>
</html>`

var badGatewayHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>502 Bad Gateway</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="c">
<h1>502 Bad Gateway</h1>
<p>The service on <strong>%s</strong> is not responding.</p>
<p>It may still be starting up or has encountered an error.</p>
<a class="btn" href="%s">Go to idapt</a>
</div>
</body>
</html>`

var portNotOpenHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Port Not Accessible</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="c">
<h1>Port Not Accessible</h1>
<p>Port <strong>%d</strong> on <strong>%s</strong> is not open or not configured in the firewall.</p>
<p>Configure the firewall in the idapt app to expose this port.</p>
<a class="btn" href="%s">Manage Machine</a>
</div>
</body>
</html>`

var serviceUnavailableHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Service Unavailable</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="c">
<h1>Service Unavailable</h1>
<p>The machine <strong>%s</strong> is temporarily unable to handle requests.</p>
<p>The agent may be overloaded or restarting. Please try again in a moment.</p>
<a class="btn" href="%s">Go to idapt</a>
</div>
</body>
</html>`
