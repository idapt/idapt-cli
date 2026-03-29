package cmdutil

import (
	"context"
	"errors"
	"testing"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/spf13/cobra"
)

func TestAPIClient_LazyInit(t *testing.T) {
	callCount := 0
	f := &Factory{}
	f.SetClientFn(func() (*api.Client, error) {
		callCount++
		return &api.Client{}, nil
	})

	c1, err := f.APIClient()
	if err != nil {
		t.Fatal(err)
	}
	if c1 == nil {
		t.Fatal("first call returned nil client")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 init call, got %d", callCount)
	}

	c2, err := f.APIClient()
	if err != nil {
		t.Fatal(err)
	}
	if c2 != c1 {
		t.Fatal("second call returned different client, expected cached")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 init call (cached), got %d", callCount)
	}
}

func TestAPIClient_ErrorPropagated(t *testing.T) {
	f := &Factory{}
	f.SetClientFn(func() (*api.Client, error) {
		return nil, errors.New("auth failed")
	})

	_, err := f.APIClient()
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if err.Error() != "auth failed" {
		t.Fatalf("error = %q, want %q", err.Error(), "auth failed")
	}
}

func TestAPIClient_NoClientFn(t *testing.T) {
	f := &Factory{}
	c, err := f.APIClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Fatal("expected nil client when no clientFn set")
	}
}

func TestSetFactory_FactoryFromCmd_RoundTrip(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	original := &Factory{NoColor: true}
	SetFactory(cmd, original)

	retrieved := FactoryFromCmd(cmd)
	if retrieved == nil {
		t.Fatal("FactoryFromCmd returned nil")
	}
	if retrieved != original {
		t.Fatal("FactoryFromCmd returned different factory")
	}
	if !retrieved.NoColor {
		t.Fatal("NoColor = false, want true")
	}
}

func TestFactoryFromCmd_NoFactory(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	f := FactoryFromCmd(cmd)
	if f != nil {
		t.Fatalf("expected nil when no factory set, got %v", f)
	}
}
