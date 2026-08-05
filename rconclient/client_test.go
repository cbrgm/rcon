package rconclient

import (
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
)

func TestClientExecute(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("ping", "pong")
	out, err := New().Execute(t.Context(), srv.Addr(), "secret", "ping")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "pong" {
		t.Fatalf("got %q", out)
	}
}

func TestPackageExecuteUsesDefaultClient(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("ping", "pong")
	out, err := Execute(t.Context(), srv.Addr(), "secret", "ping")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "pong" {
		t.Fatalf("got %q", out)
	}
}
