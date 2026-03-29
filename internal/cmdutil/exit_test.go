package cmdutil

import (
	"errors"
	"testing"

	"github.com/idapt/idapt-cli/internal/api"
)

func TestExitCodeForError_APIError_401(t *testing.T) {
	err := &api.APIError{StatusCode: 401}
	code := ExitCodeForError(err)
	if code != api.ExitAuth {
		t.Fatalf("ExitCodeForError(401) = %d, want %d", code, api.ExitAuth)
	}
}

func TestExitCodeForError_APIError_404(t *testing.T) {
	err := &api.APIError{StatusCode: 404}
	code := ExitCodeForError(err)
	if code != api.ExitNotFound {
		t.Fatalf("ExitCodeForError(404) = %d, want %d", code, api.ExitNotFound)
	}
}

func TestExitCodeForError_APIError_429(t *testing.T) {
	err := &api.APIError{StatusCode: 429}
	code := ExitCodeForError(err)
	if code != api.ExitRateLimit {
		t.Fatalf("ExitCodeForError(429) = %d, want %d", code, api.ExitRateLimit)
	}
}

func TestExitCodeForError_APIError_403(t *testing.T) {
	err := &api.APIError{StatusCode: 403}
	code := ExitCodeForError(err)
	if code != api.ExitForbidden {
		t.Fatalf("ExitCodeForError(403) = %d, want %d", code, api.ExitForbidden)
	}
}

func TestExitCodeForError_APIError_422(t *testing.T) {
	err := &api.APIError{StatusCode: 422}
	code := ExitCodeForError(err)
	if code != api.ExitValidation {
		t.Fatalf("ExitCodeForError(422) = %d, want %d", code, api.ExitValidation)
	}
}

func TestExitCodeForError_GenericError(t *testing.T) {
	err := errors.New("something broke")
	code := ExitCodeForError(err)
	if code != api.ExitError {
		t.Fatalf("ExitCodeForError(generic) = %d, want %d", code, api.ExitError)
	}
}

func TestExitCodeForError_Nil(t *testing.T) {
	code := ExitCodeForError(nil)
	if code != api.ExitOK {
		t.Fatalf("ExitCodeForError(nil) = %d, want %d", code, api.ExitOK)
	}
}
