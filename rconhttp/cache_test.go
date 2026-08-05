package rconhttp

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

func testCache(t *testing.T) *sessionCache {
	t.Helper()
	c := newCache(rconclient.New(), time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = c.closeAll() })
	return c
}

func TestCacheExecuteAndReuse(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	c := testCache(t)
	b := Backend{Addr: srv.Addr(), Password: "secret"}

	for i := 0; i < 3; i++ {
		out, err := c.execute(t.Context(), b, "list")
		if err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
		if out != "3 players" {
			t.Fatalf("out = %q", out)
		}
	}
	if n := len(c.entries); n != 1 {
		t.Fatalf("entries = %d, want 1 (session reused)", n)
	}
}

func TestCacheTwoBackends(t *testing.T) {
	a := fakercon.Start(t, "pa")
	a.Handle("who", "A")
	bsrv := fakercon.Start(t, "pb")
	bsrv.Handle("who", "B")
	c := testCache(t)

	outA, err := c.execute(t.Context(), Backend{Addr: a.Addr(), Password: "pa"}, "who")
	if err != nil || outA != "A" {
		t.Fatalf("A: %q %v", outA, err)
	}
	outB, err := c.execute(t.Context(), Backend{Addr: bsrv.Addr(), Password: "pb"}, "who")
	if err != nil || outB != "B" {
		t.Fatalf("B: %q %v", outB, err)
	}
	if n := len(c.entries); n != 2 {
		t.Fatalf("entries = %d, want 2", n)
	}
}

func TestCacheBackendDown(t *testing.T) {
	c := testCache(t)
	_, err := c.execute(t.Context(), Backend{Addr: "127.0.0.1:1", Password: "x"}, "list")
	if err == nil {
		t.Fatal("expected dial error against a dead backend")
	}
}

func TestCacheReapEvictsIdle(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "ok")
	c := newCache(rconclient.New(), 20*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer c.closeAll()

	if _, err := c.execute(t.Context(), Backend{Addr: srv.Addr(), Password: "secret"}, "list"); err != nil {
		t.Fatal(err)
	}
	if len(c.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(c.entries))
	}
	// Force the single entry to look idle, then reap.
	c.mu.Lock()
	for _, e := range c.entries {
		e.lastUsed = time.Now().Add(-time.Hour)
	}
	c.mu.Unlock()
	c.reap()
	if len(c.entries) != 0 {
		t.Fatalf("entries = %d after reap, want 0", len(c.entries))
	}
}

func TestIsConnError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"response mismatch", rcon.ErrResponseMismatch, true},
		{"eof", io.EOF, true},
		{"closed", rcon.ErrClosed, true},
		{"net error", &net.OpError{Op: "dial", Err: errors.New("x")}, true},
		{"command too long", rcon.ErrCommandTooLong, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isConnError(c.err); got != c.want {
				t.Fatalf("isConnError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestCacheConcurrent(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("ping", "pong")
	c := testCache(t)
	b := Backend{Addr: srv.Addr(), Password: "secret"}

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if out, err := c.execute(t.Context(), b, "ping"); err != nil || out != "pong" {
				t.Errorf("execute: %q %v", out, err)
			}
		})
	}
	wg.Wait()
}
