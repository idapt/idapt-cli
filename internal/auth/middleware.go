package auth

import (
	"context"
	"net/http"
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
}

// NewMiddleware creates auth middleware.
// portAuth determines which ports are public (no auth required) — typically the proxy config.
func NewMiddleware(jwt *JWTValidator, apiKey *APIKeyValidator, portAuth PublicPortChecker, pages *errorpages.Pages) *Middleware {
	return &Middleware{
		jwt:      jwt,
		apiKey:   apiKey,
		portAuth: portAuth,
		pages:    pages,
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

		// No valid auth — serve unauthenticated error page
		m.pages.ServeUnauthenticated(w, r)
	}
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
