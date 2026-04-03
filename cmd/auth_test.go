package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestAuthLoginAPIKey(t *testing.T) {
	t.Run("valid API key with uk_ prefix", func(t *testing.T) {
		// auth login --api-key uses credential.Save, which writes to disk.
		// We test the validation path: a valid prefix should print success.
		// Since credential.Save writes to DefaultPath, this test only validates
		// the command flow up to that point by checking it does not error on prefix.
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		stdout, _, err := runCmd(t, handler, "auth", "login", "--api-key", "uk_test123abc")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "API key saved") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	t.Run("invalid API key prefix rejected", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "auth", "login", "--api-key", "sk_badprefix")
		if err == nil {
			t.Fatal("expected error for invalid prefix")
		}
		if !strings.Contains(err.Error(), "API key must start with") {
			t.Errorf("expected prefix error, got: %v", err)
		}
	})

	t.Run("empty API key requires email", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "auth", "login")
		if err == nil {
			t.Fatal("expected error when no flags given")
		}
		if !strings.Contains(err.Error(), "--email is required") {
			t.Errorf("expected email required error, got: %v", err)
		}
	})
}

func TestAuthLoginEmailPassword(t *testing.T) {
	t.Run("successful login", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/auth/sign-in/email": func(w http.ResponseWriter, r *http.Request) {
				body := readJSONBody(t, r)
				if body["email"] != "user@test.com" {
					t.Errorf("expected email user@test.com, got %v", body["email"])
				}
				if body["password"] != "secret123" {
					t.Errorf("expected password secret123, got %v", body["password"])
				}
				jsonResponse(w, 200, map[string]interface{}{"token": "tok_123"})
			},
		})
		stdout, _, err := runCmd(t, handler, "auth", "login", "--email", "user@test.com", "--password", "secret123")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Login successful") {
			t.Errorf("expected success message, got: %s", stdout)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, handler, "auth", "login", "--email", "user@test.com")
		if err == nil {
			t.Fatal("expected error for missing password")
		}
		if !strings.Contains(err.Error(), "--password is required") {
			t.Errorf("expected password error, got: %v", err)
		}
	})

	t.Run("server error propagated", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/auth/sign-in/email": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 401, "Invalid credentials")
			},
		})
		_, _, err := runCmd(t, handler, "auth", "login", "--email", "user@test.com", "--password", "wrong")
		if err == nil {
			t.Fatal("expected error for invalid credentials")
		}
		if !strings.Contains(err.Error(), "Invalid credentials") {
			t.Errorf("expected credential error, got: %v", err)
		}
	})
}

func TestAuthLogout(t *testing.T) {
	t.Run("logout prints confirmation", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		stdout, _, err := runCmd(t, handler, "auth", "logout")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(stdout, "Logged out") {
			t.Errorf("expected logout message, got: %s", stdout)
		}
	})
}

func TestAuthStatus(t *testing.T) {
	t.Run("authenticated user shown", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/auth/account": func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") != "test-key" {
					t.Errorf("expected x-api-key header, got: %s", r.Header.Get("x-api-key"))
				}
				jsonResponse(w, 200, map[string]interface{}{
					"id":    "user-123",
					"email": "test@example.com",
					"name":  "Test User",
				})
			},
		})
		stdout, _, err := runCmd(t, handler, "auth", "status")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		parsed := parseJSONOutput(t, stdout)
		if parsed["email"] != "test@example.com" {
			t.Errorf("expected email in output, got: %v", parsed)
		}
	})

	t.Run("unauthenticated returns error", func(t *testing.T) {
		handler := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/auth/account": func(w http.ResponseWriter, r *http.Request) {
				jsonErrorResponse(w, 401, "Unauthorized")
			},
		})
		_, _, err := runCmd(t, handler, "auth", "status")
		if err == nil {
			t.Fatal("expected error for unauthenticated status")
		}
		if !strings.Contains(err.Error(), "not authenticated") {
			t.Errorf("expected 'not authenticated' error, got: %v", err)
		}
	})
}
