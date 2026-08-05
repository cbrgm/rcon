package rconclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rcon"
)

func TestExponentialBackoffCaps(t *testing.T) {
	b := ExponentialBackoff(100*time.Millisecond, 400*time.Millisecond)
	if d := b(10); d > 400*time.Millisecond {
		t.Fatalf("backoff not capped: %v", d)
	}
}

func TestExecuteDoesNotRetryAuthFailure(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	c := New(WithRetry(3, ExponentialBackoff(time.Millisecond, time.Millisecond)))
	_, err := c.Execute(t.Context(), srv.Addr(), "wrong", "x")
	if !errors.Is(err, rcon.ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed (no retry)", err)
	}
}

func TestExecuteRetriesDialFailure(t *testing.T) {
	// Nothing listening on this address; dial fails, retry exhausts, returns err.
	c := New(WithRetry(2, ExponentialBackoff(time.Millisecond, time.Millisecond)))
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	_, err := c.Execute(ctx, "127.0.0.1:1", "pw", "x")
	if err == nil {
		t.Fatal("expected dial error")
	}
}

// TestExecuteAppliesTimeout verifies WithTimeout bounds the whole Execute call
// (including retries) rather than being stored and ignored.
func TestExecuteAppliesTimeout(t *testing.T) {
	c := New(
		WithTimeout(20*time.Millisecond),
		WithRetry(5, ExponentialBackoff(50*time.Millisecond, 50*time.Millisecond)),
	)
	start := time.Now()
	// 192.0.2.1 is a TEST-NET-1 address (RFC 5737): reserved for documentation,
	// so dials to it black-hole rather than refusing, giving a deterministic
	// hang for the dial timeout / context deadline to cut short.
	_, err := c.Execute(t.Context(), "192.0.2.1:12345", "pw", "x")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error from bounded context")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Execute took %v, want it bounded by WithTimeout", elapsed)
	}
}
