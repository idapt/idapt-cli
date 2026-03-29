package cmdutil

import (
	"bytes"
	"strings"
	"testing"
)

func confirmFactory(input string) *Factory {
	return &Factory{
		In:     strings.NewReader(input),
		ErrOut: &bytes.Buffer{},
	}
}

func TestConfirmAction_Yes(t *testing.T) {
	if !ConfirmAction(confirmFactory("y\n"), "Delete?") {
		t.Fatal("expected true for 'y'")
	}
}

func TestConfirmAction_YesFull(t *testing.T) {
	if !ConfirmAction(confirmFactory("yes\n"), "Delete?") {
		t.Fatal("expected true for 'yes'")
	}
}

func TestConfirmAction_YesUppercase(t *testing.T) {
	if !ConfirmAction(confirmFactory("Y\n"), "Delete?") {
		t.Fatal("expected true for 'Y' (case-insensitive)")
	}
}

func TestConfirmAction_No(t *testing.T) {
	if ConfirmAction(confirmFactory("n\n"), "Delete?") {
		t.Fatal("expected false for 'n'")
	}
}

func TestConfirmAction_NoFull(t *testing.T) {
	if ConfirmAction(confirmFactory("no\n"), "Delete?") {
		t.Fatal("expected false for 'no'")
	}
}

func TestConfirmAction_EmptyInput(t *testing.T) {
	if ConfirmAction(confirmFactory("\n"), "Delete?") {
		t.Fatal("expected false for empty input (default deny)")
	}
}

func TestConfirmAction_NilFactory(t *testing.T) {
	if ConfirmAction(nil, "Delete?") {
		t.Fatal("expected false for nil factory")
	}
}

func TestConfirmAction_NilStdin(t *testing.T) {
	f := &Factory{
		In:     nil,
		ErrOut: &bytes.Buffer{},
	}
	if ConfirmAction(f, "Delete?") {
		t.Fatal("expected false for nil stdin")
	}
}
