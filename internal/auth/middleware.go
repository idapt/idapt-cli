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

type PublicPortChecker interface {
	IsPortPublic(port int) bool
}

type contextKey string

const claimsKey contextKey = "claims"

type Middleware struct {
	jwt         *JWTValidator
	portAuth    PublicPortChecker
	pages       *errorpages.Pages
	domain      string // Machine domain (e.g., "my-machine.idapt.app")
	appURL      string // App URL for auth redirects (e.g., "https://idapt.ai")
	jwksFetcher *JWKSFetcher // for on-demand JWKS refresh when JWT validation fails
}

func NewMiddleware(jwt *JWTValidator, portAuth PublicPortChecker, pages *errorpages.Pages, domain string, appURL string) *Middleware {
	return &Middleware{
		jwt:      jwt,
		portAuth: portAuth,
		pages:    pages,
		domain:   domain,
		appURL:   appURL,
	}
}

func (m *Middleware) SetJWKSFetcher(fetcher *JWKSFetcher) {
	m.jwksFetcher = fetcher
}

func (m *Middleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			next(w, r)
			return
		}

		if r.URL.Path == "/api/health" {
			next(w, r)
			return
		}

		requestPort := extractPort(r)
		if m.portAuth.IsPortPublic(requestPort) {
			next(w, r)
			return
		}

		if r.URL.Path == "/__auth_callback" {
			m.handleAuthCallback(w, r)
			return
		}

		if cookie, err := r.Cookie("idapt_machine_token"); err == nil && cookie.Value != "" {
			claims, err := m.jwt.Validate(cookie.Value)
			if err == nil {
				ctx := context.WithValue(r.Context(), claimsKey, claims)
				next(w, r.WithContext(ctx))
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			next(w, r)
			return
		}

		if isBrowserRequest(r) {
			m.redirectToAuth(w, r)
			return
		}
		m.pages.ServeUnauthenticated(w, r)
	}
}

func (m *Middleware) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	redirectPath := r.URL.Query().Get("path")

	if token == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}

	if _, err := m.jwt.Validate(token); err != nil {
		if m.jwksFetcher != nil {
			if refreshErr := m.jwksFetcher.RefreshNow(); refreshErr == nil {
				if _, retryErr := m.jwt.Validate(token); retryErr == nil {
					goto setCookie
				}
			}
		}
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
setCookie:

	redirectPath = sanitizeRedirectPath(redirectPath)

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

	if port := listener.ListenerPortFromContext(r.Context()); port > 0 {
		authURL += fmt.Sprintf("&port=%d", port)
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func sanitizeRedirectPath(path string) string {
	if path == "" {
		return "/"
	}

	if !strings.HasPrefix(path, "/") {
		return "/"
	}

	if strings.HasPrefix(path, "//") {
		return "/"
	}

	if strings.Contains(path, "\\") {
		return "/"
	}

	if strings.ContainsRune(path, 0) {
		path = strings.ReplaceAll(path, "\x00", "")
	}

	if path == "" {
		return "/"
	}

	return path
}

func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

func extractSlug(domain string) string {
	host := domain
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	parts := strings.SplitN(host, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return domain
}

func GetClaims(r *http.Request) *Claims {
	claims, _ := r.Context().Value(claimsKey).(*Claims)
	return claims
}

func extractPort(r *http.Request) int {
	if port := listener.ListenerPortFromContext(r.Context()); port > 0 {
		return port
	}
	return 443
}
