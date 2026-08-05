package rconclient

import (
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
)

// TestSessionReconnectOnce drives the Session retry path: the fake server drops
// the first command mid-session (a retryable io.EOF), and the Session must
// reconnect once and succeed on the retry.
func TestSessionReconnectOnce(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("ping", "pong")
	s, err := New().Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer s.Close()

	srv.DropNext() // first Execute hits a dropped connection
	got, err := s.Execute(t.Context(), "ping")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "pong" {
		t.Fatalf("got %q, want %q", got, "pong")
	}
}

func TestSessionExecute(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("a", "1")
	srv.Handle("b", "2")
	s, err := New().Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer s.Close()
	for _, tc := range []struct{ cmd, want string }{{"a", "1"}, {"b", "2"}} {
		got, err := s.Execute(t.Context(), tc.cmd)
		if err != nil {
			t.Fatalf("Execute %q: %v", tc.cmd, err)
		}
		if got != tc.want {
			t.Fatalf("cmd %q = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}
