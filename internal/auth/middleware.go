package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/idapt/idapt-cli/internal/errorpages"
	"github.com/idapt/idapt-cli/internal/listener"
)

// PublicPortChecker determines if a port allows unauthenticated access.
// Implemented by proxy.ConfigManager (checks proxy authMode, not firewall).
type PublicPortChecker interface {
	IsPortPublic(port int) bool
}

type contextKey string

const claimsKey contextKey = "claims"

// Middleware handles authentication for incoming requests.
type Middleware struct {
	jwt      *JWTValidator
	apiKey   *APIKeyValidator
	portAuth PublicPortChecker
	pages    *errorpages.Pages
	domain   string // Machine domain (e.g., "my-machine.idapt.app")
	appURL   string // App URL for auth redirects (e.g., "https://idapt.ai")
}

// NewMiddleware creates auth middleware.
// portAuth determines which ports are public (no auth required) — typically the proxy config.
// domain is the machine's subdomain (e.g., "my-machine.idapt.app").
// appURL is the idapt app URL for auth redirects (e.g., "https://idapt.ai").
func NewMiddleware(jwt *JWTValidator, apiKey *APIKeyValidator, portAuth PublicPortChecker, pages *errorpages.Pages, domain string, appURL string) *Middleware {
	return &Middleware{
		jwt:      jwt,
		apiKey:   apiKey,
		portAuth: portAuth,
		pages:    pages,
		domain:   domain,
		appURL:   appURL,
	}
}

// Wrap returns an http.HandlerFunc that enforces authentication before
// calling the wrapped handler.
func (m *Middleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Always allow ACME challenges (certmagic needs these)
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			next(w, r)
			return
		}

		// Always allow health checks
		if r.URL.Path == "/api/health" {
			next(w, r)
			return
		}

		// Check if the requested port is public (no auth required).
		// Uses the proxy config's authMode, not the firewall.
		requestPort := extractPort(r)
		if m.portAuth.IsPortPublic(requestPort) {
			next(w, r)
			return
		}

		// Handle auth callback: validate JWT token, set cookie, redirect to path.
		// This must be checked BEFORE cookie/Bearer auth to prevent the callback
		// path from being treated as a normal authenticated request.
		if r.URL.Path == "/__auth_callback" {
			m.handleAuthCallback(w, r)
			return
		}

		// Try JWT cookie
		if cookie, err := r.Cookie("idapt_machine_token"); err == nil && cookie.Value != "" {
			claims, err := m.jwt.Validate(cookie.Value)
			if err == nil {
				ctx := context.WithValue(r.Context(), claimsKey, claims)
				next(w, r.WithContext(ctx))
				return
			}
			// JWT invalid — fall through to try API key
		}

		// Try Bearer token (API key)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(token, APIKeyPrefix) {
				if err := m.apiKey.Validate(token); err == nil {
					next(w, r)
					return
				}
			}
		}

		// No valid auth — redirect browsers to auth endpoint, return 401 for API clients
		if isBrowserRequest(r) {
			m.redirectToAuth(w, r)
			return
		}
		m.pages.ServeUnauthenticated(w, r)
	}
}

// handleAuthCallback processes the /__auth_callback endpoint:
// validates the JWT token, sets an HttpOnly cookie, and redirects to the original path.
func (m *Middleware) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	redirectPath := r.URL.Query().Get("path")

	if token == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}

	// Validate the JWT before setting cookie (prevent storing garbage)
	if _, err := m.jwt.Validate(token); err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Sanitize redirect path to prevent open redirects
	redirectPath = sanitizeRedirectPath(redirectPath)

	// Determine cookie properties
	isLocalhost := strings.Contains(m.domain, "localhost")
	cookieDomain := m.domain
	if idx := strings.Index(cookieDomain, ":"); idx != -1 {
		cookieDomain = cookieDomain[:idx] // Strip port from domain
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "idapt_machine_token",
		Value:    token,
		Path:     "/",
		Domain:   cookieDomain,
		HttpOnly: true,
		Secure:   !isLocalhost,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours, matching JWT expiry
	})

	http.Redirect(w, r, redirectPath, http.StatusFound)
}

// redirectToAuth sends a 302 redirect to the idapt.ai auth endpoint
// with the machine slug and original path as query parameters.
func (m *Middleware) redirectToAuth(w http.ResponseWriter, r *http.Request) {
	slug := extractSlug(m.domain)
	originalPath := r.URL.Path
	if r.URL.RawQuery != "" {
		originalPath += "?" + r.URL.RawQuery
	}

	authURL := fmt.Sprintf("%s/api/managed-machines/auth?slug=%s&path=%s",
		m.appURL,
		url.QueryEscape(slug),
		url.QueryEscape(originalPath),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// sanitizeRedirectPath ensures the path is a safe relative path.
// Rejects absolute URLs, protocol-relative URLs, javascript:, data:, and backslash tricks.
func sanitizeRedirectPath(path string) string {
	if path == "" {
		return "/"
	}

	// Reject non-relative paths (absolute URLs, protocol-relative, javascript:, data:, etc.)
	if !strings.HasPrefix(path, "/") {
		return "/"
	}

	// Reject protocol-relative URLs (//evil.com)
	if strings.HasPrefix(path, "//") {
		return "/"
	}

	// Reject backslash tricks (\evil.com)
	if strings.Contains(path, "\\") {
		return "/"
	}

	// Strip null bytes
	if strings.ContainsRune(path, 0) {
		path = strings.ReplaceAll(path, "\x00", "")
	}

	if path == "" {
		return "/"
	}

	return path
}

// isBrowserRequest checks if the request appears to come from a web browser
// based on the Accept header containing text/html.
func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

// extractSlug extracts the machine slug from the domain.
// "my-machine.idapt.app" → "my-machine"
// "my-machine.localhost:8443" → "my-machine"
func extractSlug(domain string) string {
	// Strip port first
	host := domain
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// First label is the slug
	parts := strings.SplitN(host, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return domain
}

// GetClaims extracts JWT claims from the request context.
func GetClaims(r *http.Request) *Claims {
	claims, _ := r.Context().Value(claimsKey).(*Claims)
	return claims
}

func extractPort(r *http.Request) int {
	// Dynamic listeners inject their port into the request context.
	// For the main :443 listener, this returns 0 so we default to 443.
	if port := listener.ListenerPortFromContext(r.Context()); port > 0 {
		return port
	}
	return 443
}
