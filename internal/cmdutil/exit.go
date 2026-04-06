package cmdutil

import (
	"os"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/output"
)

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

func ExitCodeForError(err error) int {
	if err == nil {
		return api.ExitOK
	}
	if apiErr, ok := err.(*api.APIError); ok {
		return apiErr.ExitCode()
	}
	return api.ExitError
}
