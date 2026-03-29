package cmdutil

import (
	"os"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/output"
)

// ExitWithError writes the error to stderr and exits with the appropriate code.
func ExitWithError(f *Factory, err error) {
	if err == nil {
		return
	}

	output.WriteError(f.Format, f.ErrOut, err)

	if apiErr, ok := err.(*api.APIError); ok {
		os.Exit(apiErr.ExitCode())
	}
	os.Exit(api.ExitError)
}

// ExitCodeForError returns the exit code for an error without exiting.
func ExitCodeForError(err error) int {
	if err == nil {
		return api.ExitOK
	}
	if apiErr, ok := err.(*api.APIError); ok {
		return apiErr.ExitCode()
	}
	return api.ExitError
}
