// Package cmdutil provides shared command utilities and dependency injection.
package cmdutil

import (
	"context"
	"io"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cliconfig"
	"github.com/idapt/idapt-cli/internal/credential"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/idapt/idapt-cli/internal/resolve"
	"github.com/spf13/cobra"
)

type contextKey struct{}

// Factory provides shared dependencies to all commands.
type Factory struct {
	Config      cliconfig.Config
	Credentials credential.Credentials
	Format      output.Format
	NoColor     bool
	Out         io.Writer
	ErrOut      io.Writer
	In          io.Reader

	client   *api.Client
	clientFn func() (*api.Client, error)
	resolver *resolve.Resolver
}

// SetClientFn sets the lazy client constructor.
func (f *Factory) SetClientFn(fn func() (*api.Client, error)) {
	f.clientFn = fn
}

// APIClient returns the API client, initializing it lazily.
func (f *Factory) APIClient() (*api.Client, error) {
	if f.client != nil {
		return f.client, nil
	}
	if f.clientFn == nil {
		return nil, nil
	}
	c, err := f.clientFn()
	if err != nil {
		return nil, err
	}
	f.client = c
	return c, nil
}

// Resolver returns the resource resolver, initializing it lazily.
func (f *Factory) Resolver() (*resolve.Resolver, error) {
	if f.resolver != nil {
		return f.resolver, nil
	}
	c, err := f.APIClient()
	if err != nil {
		return nil, err
	}
	f.resolver = resolve.New(c)
	return f.resolver, nil
}

// Formatter creates a formatter for the current output format.
func (f *Factory) Formatter() output.Formatter {
	return output.New(f.Format, f.Out, f.NoColor)
}

// SetFactory stores a Factory in the command context.
func SetFactory(cmd *cobra.Command, f *Factory) {
	cmd.SetContext(context.WithValue(cmd.Context(), contextKey{}, f))
}

// FactoryFromCmd retrieves the Factory from a command context.
func FactoryFromCmd(cmd *cobra.Command) *Factory {
	f, _ := cmd.Context().Value(contextKey{}).(*Factory)
	return f
}
