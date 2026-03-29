package errorpages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPages_ServeUnauthenticated(t *testing.T) {
	pages := New("test-machine.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	pages.ServeUnauthenticated(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	body := w.Body.String()

	// Must be >512 bytes to avoid browser/CDN interception
	if len(body) < 512 {
		t.Errorf("body length = %d, want >512", len(body))
	}

	// Must be valid HTML
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("body should be valid HTML")
	}

	// Must include domain
	if !strings.Contains(body, "test-machine.idapt.app") {
		t.Error("body should include the domain")
	}

	// Must include link to app
	if !strings.Contains(body, "https://idapt.ai") {
		t.Error("body should include link to app")
	}

	// Must not contain unescaped HTML
	if strings.Contains(body, "<script>") {
		t.Error("body should not contain script tags")
	}
}

func TestPages_XSSPrevention(t *testing.T) {
	// Test with XSS in domain
	pages := New("<script>alert(1)</script>.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	pages.ServeUnauthenticated(w, req)

	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("domain should be HTML-escaped to prevent XSS")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("domain should be properly escaped")
	}
}

func TestPages_ServeBadGateway(t *testing.T) {
	pages := New("test.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	pages.ServeBadGateway(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
	if len(w.Body.String()) < 512 {
		t.Error("body too short")
	}
}

func TestPages_ServePortNotOpen(t *testing.T) {
	pages := New("test.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	pages.ServePortNotOpen(w, req, 3000)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	body := w.Body.String()
	if !strings.Contains(body, "3000") {
		t.Error("body should include the port number")
	}
}

func TestPages_ServeServiceUnavailable(t *testing.T) {
	pages := New("test.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	pages.ServeServiceUnavailable(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if len(w.Body.String()) < 512 {
		t.Error("body too short")
	}
}

func TestPages_AllPagesHaveDarkLightTheme(t *testing.T) {
	pages := New("test.idapt.app", "https://idapt.ai")
	req := httptest.NewRequest("GET", "/", nil)

	handlers := []func(http.ResponseWriter, *http.Request){
		pages.ServeUnauthenticated,
		pages.ServeBadGateway,
		pages.ServeServiceUnavailable,
	}

	for i, handler := range handlers {
		w := httptest.NewRecorder()
		handler(w, req)
		body := w.Body.String()
		if !strings.Contains(body, "prefers-color-scheme") {
			t.Errorf("page %d should include dark/light theme support", i)
		}
	}
}
