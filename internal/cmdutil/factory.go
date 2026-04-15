package cmdutil

import (
	"context"
	"io"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cliconfig"
	"github.com/idapt/idapt-cli/internal/credential"
	"github.com/idapt/idapt-cli/internal/features"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/idapt/idapt-cli/internal/resolve"
	"github.com/spf13/cobra"
)

type contextKey struct{}

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

	flags   *features.Flags
	flagsFn func() (*features.Flags, error)
}

func (f *Factory) SetClientFn(fn func() (*api.Client, error)) {
	f.clientFn = fn
}

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

func (f *Factory) Formatter() output.Formatter {
	return output.New(f.Format, f.Out, f.NoColor)
}

func (f *Factory) SetFlagsFn(fn func() (*features.Flags, error)) {
	f.flagsFn = fn
}

func (f *Factory) Features() *features.Flags {
	if f.flags != nil {
		return f.flags
	}
	if f.flagsFn == nil {
		f.flags = &features.Flags{}
		return f.flags
	}
	flags, err := f.flagsFn()
	if err != nil || flags == nil {
		f.flags = &features.Flags{}
		return f.flags
	}
	f.flags = flags
	return f.flags
}

func SetFactory(cmd *cobra.Command, f *Factory) {
	cmd.SetContext(context.WithValue(cmd.Context(), contextKey{}, f))
}

func FactoryFromCmd(cmd *cobra.Command) *Factory {
	f, _ := cmd.Context().Value(contextKey{}).(*Factory)
	return f
}
