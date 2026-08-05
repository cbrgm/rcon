package rconclient_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rconclient"
)

func TestWithDialerUsedForExecute(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	var dialed atomic.Bool
	c := rconclient.New(rconclient.WithDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		dialed.Store(true)
		var d net.Dialer
		return d.DialContext(ctx, "tcp", srv.Addr())
	}))
	out, err := c.Execute(t.Context(), srv.Addr(), "secret", "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "3 players" {
		t.Fatalf("out = %q", out)
	}
	if !dialed.Load() {
		t.Fatal("custom dialer was not used")
	}
}

func TestWithDialerUsedForSession(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("a", "1")
	srv.Handle("b", "2")
	c := rconclient.New(rconclient.WithDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", srv.Addr())
	}))
	s, err := c.Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer s.Close()
	for _, tc := range []struct{ cmd, want string }{{"a", "1"}, {"b", "2"}} {
		got, err := s.Execute(t.Context(), tc.cmd)
		if err != nil || got != tc.want {
			t.Fatalf("Execute %q = %q, %v", tc.cmd, got, err)
		}
	}
}

func TestWithDialerErrorPropagates(t *testing.T) {
	wantErr := errors.New("dial boom")
	c := rconclient.New(rconclient.WithDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return nil, wantErr
	}))
	_, err := c.Execute(t.Context(), "x:1", "pw", "list")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestWithDialerNilConn(t *testing.T) {
	c := rconclient.New(rconclient.WithDialer(func(_ context.Context, _ string) (net.Conn, error) {
		return nil, nil // buggy dialer: nil conn, nil error
	}))
	_, err := c.Execute(t.Context(), "x:1", "pw", "list")
	if err == nil {
		t.Fatal("expected an error for a nil connection, got nil")
	}
}

func TestWithDialerClosesConnOnAuthFailure(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	var closed atomic.Bool
	c := rconclient.New(rconclient.WithDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		nc, err := d.DialContext(ctx, "tcp", srv.Addr())
		if err != nil {
			return nil, err
		}
		return &closeTracker{Conn: nc, closed: &closed}, nil
	}))
	_, err := c.Execute(t.Context(), srv.Addr(), "wrong", "list")
	if !errors.Is(err, rconclient.ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
	if !closed.Load() {
		t.Fatal("custom-dialed conn was not closed on auth failure")
	}
}

type closeTracker struct {
	net.Conn
	closed *atomic.Bool
}

func (c *closeTracker) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}
