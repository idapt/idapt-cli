package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/idapt/idapt-cli/internal/errorpages"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

const (
	mwTestMachineID = "mm-mw-test-123"
	mwTestActorID   = "actor-mw-test-456"
	mwTestDomain    = "tada.idapt.app"
	mwTestAppURL    = "https://idapt.ai"
	mwLocalDomain   = "tada.localhost:8443"
	mwCookieName    = "idapt_machine_token"
)

// mwValidClaims returns a standard set of valid JWT claims for middleware tests.
func mwValidClaims() map[string]interface{} {
	return map[string]interface{}{
		"sub": mwTestActorID,
		"mid": mwTestMachineID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

// okHandler is the wrapped handler that writes "OK" when auth succeeds.
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// newMWTestMiddleware creates a middleware for production domain tests.
func newMWTestMiddleware(t *testing.T) (*Middleware, func(t *testing.T, claims map[string]interface{}) string) {
	t.Helper()
	return newMWTestMiddlewareWithDomain(t, mwTestDomain, mwTestAppURL, nil)
}

// newMWTestMiddlewareWithDomain creates a middleware with the specified domain and optional public ports.
// Returns the middleware and a signJWT helper bound to the generated private key.
func newMWTestMiddlewareWithDomain(t *testing.T, domain, appURL string, publicPorts map[int]bool) (*Middleware, func(t *testing.T, claims map[string]interface{}) string) {
	t.Helper()

	priv, pubPEM := generateTestKeyPair(t)

	jwtValidator, err := NewJWTValidator(pubPEM, mwTestMachineID)
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}

	if publicPorts == nil {
		publicPorts = map[int]bool{}
	}
	portChecker := &mockPortChecker{publicPorts: publicPorts}

	pages := errorpages.New(domain, appURL)

	mw := NewMiddleware(jwtValidator, portChecker, pages, domain, appURL)

	signJWT := func(t *testing.T, claims map[string]interface{}) string {
		t.Helper()
		return createTestES256JWT(t, priv, claims)
	}

	return mw, signJWT
}

// newMWLocalhostMiddleware creates a middleware configured for localhost.
func newMWLocalhostMiddleware(t *testing.T) (*Middleware, func(t *testing.T, claims map[string]interface{}) string) {
	t.Helper()
	return newMWTestMiddlewareWithDomain(t, mwLocalDomain, "http://localhost:3000", nil)
}

// mockPortChecker implements PublicPortChecker for tests.
type mockPortChecker struct {
	publicPorts map[int]bool
}

func (m *mockPortChecker) IsPortPublic(port int) bool {
	return m.publicPorts[port]
}

// signHS256JWT creates an HS256 JWT (should be rejected by ES256 validator).
func signHS256JWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	// Create a fake HMAC signature (doesn't matter what key — the validator should reject alg:HS256)
	mac := hmac.New(sha256.New, []byte("arbitrary-secret"))
	mac.Write([]byte("fake-signing-input"))
	fakeSig := mac.Sum(nil)
	return createJWTWithHeader(
		t,
		[]byte(`{"alg":"HS256","typ":"JWT"}`),
		claims,
		fakeSig,
	)
}

// assertNoCookie checks that no Set-Cookie header with the given name was set.
func assertNoCookie(t *testing.T, w *httptest.ResponseRecorder, name string) {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			t.Errorf("unexpected Set-Cookie header for %q", name)
			return
		}
	}
}

// findCookie returns the cookie with the given name, or nil.
func findCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// A. Auth Callback -- Happy Path
// ---------------------------------------------------------------------------

func TestAuthCallback_ValidToken_SetsCookie(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if findCookie(w, mwCookieName) == nil {
		t.Fatal("expected Set-Cookie with name idapt_machine_token")
	}
}

func TestAuthCallback_ValidToken_RedirectsToPath(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/dashboard", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/dashboard" {
		t.Errorf("Location = %q, want %q", loc, "/dashboard")
	}
}

func TestAuthCallback_ValidToken_DefaultPath(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
}

func TestAuthCallback_ValidToken_PathWithQuery(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/app%3Ftab%3Dsettings", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/app?tab=settings" {
		t.Errorf("Location = %q, want %q", loc, "/app?tab=settings")
	}
}

func TestAuthCallback_ValidToken_PathWithHash(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/app%23section", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/app#section" {
		t.Errorf("Location = %q, want %q", loc, "/app#section")
	}
}

func TestAuthCallback_ValidToken_DeepPath(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/a/b/c/d/e", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/a/b/c/d/e" {
		t.Errorf("Location = %q, want %q", loc, "/a/b/c/d/e")
	}
}

// ---------------------------------------------------------------------------
// B. Cookie Security Properties (CRITICAL)
// ---------------------------------------------------------------------------

func TestAuthCallback_Cookie_HttpOnly(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if !c.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
}

func TestAuthCallback_Cookie_Secure_Production(t *testing.T) {
	mw, signJWT := newMWTestMiddlewareWithDomain(t, "tada.idapt.app", mwTestAppURL, nil)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if !c.Secure {
		t.Error("cookie must be Secure for production domain")
	}
}

func TestAuthCallback_Cookie_Secure_Localhost(t *testing.T) {
	mw, signJWT := newMWLocalhostMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.Secure {
		t.Error("cookie must NOT be Secure for localhost domain")
	}
}

func TestAuthCallback_Cookie_SameSite_Lax(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
}

func TestAuthCallback_Cookie_Path_Root(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want %q", c.Path, "/")
	}
}

func TestAuthCallback_Cookie_MaxAge_24h(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.MaxAge != 86400 {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, 86400)
	}
}

func TestAuthCallback_Cookie_Domain_Stripped(t *testing.T) {
	mw, signJWT := newMWTestMiddlewareWithDomain(t, "tada.idapt.app", mwTestAppURL, nil)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.Domain != "tada.idapt.app" {
		t.Errorf("Domain = %q, want %q", c.Domain, "tada.idapt.app")
	}
}

func TestAuthCallback_Cookie_Domain_LocalhostPort(t *testing.T) {
	mw, signJWT := newMWLocalhostMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	// Port should be stripped from the cookie domain
	if c.Domain != "tada.localhost" {
		t.Errorf("Domain = %q, want %q (port stripped)", c.Domain, "tada.localhost")
	}
}

func TestAuthCallback_Cookie_Name(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if findCookie(w, mwCookieName) == nil {
		t.Fatal("expected cookie named idapt_machine_token")
	}
}

func TestAuthCallback_Cookie_Value_MatchesToken(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	c := findCookie(w, mwCookieName)
	if c == nil {
		t.Fatal("cookie not found")
	}
	if c.Value != token {
		t.Errorf("cookie value does not match token\ngot:  %q\nwant: %q", c.Value, token)
	}
}

// ---------------------------------------------------------------------------
// C. Auth Callback -- Rejection
// ---------------------------------------------------------------------------

func TestAuthCallback_MissingToken(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/__auth_callback", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertNoCookie(t, w, mwCookieName)
}

func TestAuthCallback_EmptyToken(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/__auth_callback?token=", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertNoCookie(t, w, mwCookieName)
}

func TestAuthCallback_InvalidToken(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/__auth_callback?token=not-a-valid-jwt", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertNoCookie(t, w, mwCookieName)
}

func TestAuthCallback_ExpiredToken(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)

	claims := mwValidClaims()
	claims["exp"] = time.Now().Add(-1 * time.Hour).Unix()
	claims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
	token := signJWT(t, claims)

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertNoCookie(t, w, mwCookieName)
}

func TestAuthCallback_WrongMachineToken(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)

	claims := mwValidClaims()
	claims["mid"] = "mm-wrong-machine"
	token := signJWT(t, claims)

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertNoCookie(t, w, mwCookieName)
}

func TestAuthCallback_HS256Token(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	token := signHS256JWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertNoCookie(t, w, mwCookieName)
}

// ---------------------------------------------------------------------------
// D. Open Redirect Prevention (CRITICAL)
// ---------------------------------------------------------------------------

func TestAuthCallback_Path_AbsoluteURL(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=https://evil.com", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("absolute URL should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_ProtocolRelative(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=//evil.com/steal", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("protocol-relative URL should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_Javascript(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=javascript:alert(1)", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("javascript: URI should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_DataURI(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=data:text/html,<h1>pwned</h1>", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("data: URI should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_BackslashTrick(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+`&path=\evil.com`, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("backslash trick should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_Encoded(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	// %2F = /
	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=%2Fdashboard", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/dashboard" {
		t.Errorf("encoded path should decode to /dashboard, got %q", loc)
	}
}

func TestAuthCallback_Path_DoubleEncoded(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	// %252F = %2F (double-encoded) -- after one URL decode, starts with %, not /, so should fall back to /
	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=%252Fdashboard", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("double-encoded path should redirect to /, got %q", loc)
	}
}

func TestAuthCallback_Path_RelativeWithDots(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/../../../etc/passwd", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	// After path.Clean, /../../../etc/passwd becomes /etc/passwd, which starts with / and is acceptable.
	// The key assertion: no ".." traversal in the final redirect.
	if strings.Contains(loc, "..") {
		t.Errorf("path traversal should be cleaned, got %q", loc)
	}
	if !strings.HasPrefix(loc, "/") {
		t.Errorf("location should start with /, got %q", loc)
	}
}

func TestAuthCallback_Path_NullByte(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/app%00evil", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	// Null byte should either be stripped or cause fallback to /
	if strings.Contains(loc, "\x00") {
		t.Errorf("null byte should be stripped from path, got %q", loc)
	}
}

func TestAuthCallback_Path_ValidRelative(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/dashboard", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/dashboard" {
		t.Errorf("Location = %q, want %q", loc, "/dashboard")
	}
}

func TestAuthCallback_Path_RootOnly(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token+"&path=/", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
}

// ---------------------------------------------------------------------------
// E. 401 Behavior -- Browser vs API
// ---------------------------------------------------------------------------

func TestUnauth_Browser_HtmlAccept(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/some/page", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (302 redirect for browser)", w.Code, http.StatusFound)
	}
}

func TestUnauth_Browser_WildcardAccept(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/some/page", nil)
	req.Header.Set("Accept", "text/html, */*")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (302 redirect for browser)", w.Code, http.StatusFound)
	}
}

func TestUnauth_Browser_HtmlFirst(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/some/page", nil)
	req.Header.Set("Accept", "text/html, application/json")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (302 redirect for browser)", w.Code, http.StatusFound)
	}
}

func TestUnauth_API_JsonAccept(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (401 for API)", w.Code, http.StatusUnauthorized)
	}
}

func TestUnauth_API_NoAccept(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	// No Accept header
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (401 for no Accept)", w.Code, http.StatusUnauthorized)
	}
}

func TestUnauth_API_PlainText(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (401 for text/plain)", w.Code, http.StatusUnauthorized)
	}
}

func TestUnauth_Browser_RedirectURL_Format(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/some/page", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	// Redirect URL should point to the app's auth endpoint
	if !strings.HasPrefix(loc, mwTestAppURL) {
		t.Errorf("redirect URL should start with appURL %q, got %q", mwTestAppURL, loc)
	}
}

func TestUnauth_Browser_RedirectURL_Slug(t *testing.T) {
	mw, _ := newMWTestMiddlewareWithDomain(t, "my-machine.idapt.app", mwTestAppURL, nil)

	req := httptest.NewRequest("GET", "/page", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	// The redirect should include the machine slug
	if !strings.Contains(loc, "my-machine") {
		t.Errorf("redirect URL should contain machine slug 'my-machine', got %q", loc)
	}
}

func TestUnauth_Browser_RedirectURL_OriginalPath(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/original/path", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	// The original path should be preserved in the redirect URL (URL-encoded)
	if !strings.Contains(loc, "path=") || !strings.Contains(loc, "original") {
		t.Errorf("redirect URL should contain original path, got %q", loc)
	}
}

// ---------------------------------------------------------------------------
// F. Auth Bypass Paths
// ---------------------------------------------------------------------------

func TestBypass_ACMEChallenge(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/.well-known/acme-challenge/test-token-123", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (ACME should bypass auth)", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", w.Body.String(), "OK")
	}
}

func TestBypass_ACMEChallenge_SubPath(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/.well-known/acme-challenge/sub/path/token", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (ACME sub-path should bypass auth)", w.Code, http.StatusOK)
	}
}

func TestBypass_HealthCheck(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (health check should bypass auth)", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", w.Body.String(), "OK")
	}
}

func TestNotBypassed_WellKnownOther(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/.well-known/other", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	// Should require auth -- not bypassed
	if w.Code == http.StatusOK {
		t.Error("/.well-known/other should not bypass auth")
	}
}

func TestNotBypassed_RandomPath(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/anything", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("/anything should not bypass auth")
	}
}

// ---------------------------------------------------------------------------
// G. Cookie Authentication
// ---------------------------------------------------------------------------

func TestCookieAuth_ValidCookie(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: token})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (valid cookie should pass through)", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", w.Body.String(), "OK")
	}
}

func TestCookieAuth_ExpiredCookie(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)

	claims := mwValidClaims()
	claims["exp"] = time.Now().Add(-1 * time.Hour).Unix()
	token := signJWT(t, claims)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: token})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("expired cookie should not pass through")
	}
}

func TestCookieAuth_MalformedCookie(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: "not-a-jwt"})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("malformed cookie should not pass through")
	}
}

func TestCookieAuth_EmptyCookie(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: ""})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("empty cookie should not pass through")
	}
}

func TestCookieAuth_WrongCookieName(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: "wrong_cookie_name", Value: token})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("wrong cookie name should not pass through")
	}
}

func TestCookieAuth_ClaimsInContext(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	var gotClaims *Claims
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotClaims = GetClaims(r)
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: token})
	w := httptest.NewRecorder()

	mw.Wrap(handler)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotClaims == nil {
		t.Fatal("expected claims in context, got nil")
	}
	if gotClaims.Sub != mwTestActorID {
		t.Errorf("Sub = %q, want %q", gotClaims.Sub, mwTestActorID)
	}
	if gotClaims.Mid != mwTestMachineID {
		t.Errorf("Mid = %q, want %q", gotClaims.Mid, mwTestMachineID)
	}
}

// ---------------------------------------------------------------------------
// H. Bearer Token Transparent Passthrough
// ---------------------------------------------------------------------------

func TestBearer_AnyToken_PassesThrough(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer uk_some-user-key")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (any Bearer token should pass through)", w.Code, http.StatusOK)
	}
}

func TestBearer_AKToken_PassesThrough(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer ak_agent-key-123")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (ak_ Bearer token should pass through)", w.Code, http.StatusOK)
	}
}

func TestBearer_PKToken_PassesThrough(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer pk_project-key-456")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (pk_ Bearer token should pass through)", w.Code, http.StatusOK)
	}
}

func TestBearer_ArbitraryToken_PassesThrough(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer some-arbitrary-token-value")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (arbitrary Bearer token should pass through)", w.Code, http.StatusOK)
	}
}

func TestBearer_EmptyAfterPrefix_PassesThrough(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	// "Bearer " with nothing after — still has the prefix, passes through
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (Bearer with empty token should pass through)", w.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// I. Public Port
// ---------------------------------------------------------------------------

func TestPublicPort_Configured(t *testing.T) {
	// Default port for requests without listener context is 443.
	// Set port 443 as public to test the bypass.
	mw, _ := newMWTestMiddlewareWithDomain(t, mwTestDomain, mwTestAppURL, map[int]bool{443: true})

	req := httptest.NewRequest("GET", "/anything", nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (public port should bypass auth)", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", w.Body.String(), "OK")
	}
}

func TestPublicPort_NotConfigured(t *testing.T) {
	// Port 8080 is public, but requests default to port 443 which is not public.
	mw, _ := newMWTestMiddlewareWithDomain(t, mwTestDomain, mwTestAppURL, map[int]bool{8080: true})

	req := httptest.NewRequest("GET", "/anything", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("non-public port should require auth")
	}
}

// ---------------------------------------------------------------------------
// J. Priority
// ---------------------------------------------------------------------------

func TestPriority_CallbackBeforeCookie(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	callbackToken := signJWT(t, mwValidClaims())
	cookieToken := signJWT(t, mwValidClaims())

	// Request to __auth_callback with a valid cookie already set
	req := httptest.NewRequest("GET", "/__auth_callback?token="+callbackToken+"&path=/target", nil)
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: cookieToken})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	// The callback handler should run (302 redirect), not the cookie auth (200 pass-through)
	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (callback should take priority over cookie)", w.Code, http.StatusFound)
	}
}

func TestPriority_CookieBeforeBearer(t *testing.T) {
	mw, signJWT := newMWTestMiddleware(t)
	token := signJWT(t, mwValidClaims())

	var gotClaims *Claims
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotClaims = GetClaims(r)
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: token})
	req.Header.Set("Authorization", "Bearer uk_some-key")
	w := httptest.NewRecorder()

	mw.Wrap(handler)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	// When cookie auth succeeds, claims should be in context (Bearer passthrough doesn't set claims)
	if gotClaims == nil {
		t.Fatal("expected claims from cookie auth, got nil (Bearer was used instead)")
	}
	if gotClaims.Sub != mwTestActorID {
		t.Errorf("Sub = %q, want %q", gotClaims.Sub, mwTestActorID)
	}
}

func TestPriority_BearerWhenCookieInvalid(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: "invalid-jwt-token"})
	req.Header.Set("Authorization", "Bearer uk_some-key")
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (Bearer should work when cookie is invalid)", w.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

// Verify that a token signed with a different ECDSA key is rejected by cookie auth.
func TestCookieAuth_WrongSigningKey(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)
	otherPriv, _ := generateTestKeyPair(t)
	token := createTestES256JWT(t, otherPriv, mwValidClaims())

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: mwCookieName, Value: token})
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code == http.StatusOK {
		t.Error("token signed with wrong key should not pass through")
	}
}

// Verify that the auth callback rejects tokens signed with a different key.
func TestAuthCallback_WrongSigningKey(t *testing.T) {
	mw, _ := newMWTestMiddleware(t)
	otherPriv, _ := generateTestKeyPair(t)
	token := createTestES256JWT(t, otherPriv, mwValidClaims())

	req := httptest.NewRequest("GET", "/__auth_callback?token="+token, nil)
	w := httptest.NewRecorder()

	mw.Wrap(okHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertNoCookie(t, w, mwCookieName)
}
