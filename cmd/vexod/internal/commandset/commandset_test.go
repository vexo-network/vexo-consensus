package commandset

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestRegistryRunsKnownCommand(t *testing.T) {
	registry := New([]Command{
		{
			Name: "hello",
			Handler: func(writer io.Writer, args []string) error {
				_, _ = writer.Write([]byte(args[0]))
				return nil
			},
		},
	})
	var output bytes.Buffer

	handled, err := registry.Run("hello", &output, []string{"world"})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if !handled {
		t.Fatalf("expected command to be handled")
	}
	if output.String() != "world" {
		t.Fatalf("unexpected output %q", output.String())
	}
}

func TestRegistryReportsUnknownCommand(t *testing.T) {
	registry := New(nil)

	handled, err := registry.Run("missing", &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected unknown command to be unhandled")
	}
}

func TestRegistryPropagatesHandlerError(t *testing.T) {
	expected := errors.New("boom")
	registry := New([]Command{
		{Name: "fail", Handler: func(_ io.Writer, _ []string) error {
			return expected
		}},
	})

	handled, err := registry.Run("fail", &bytes.Buffer{}, nil)
	if !handled {
		t.Fatalf("expected command to be handled")
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}
