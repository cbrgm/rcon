package rconclient

import (
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// ExponentialBackoff returns a BackoffFunc that doubles from base up to max,
// with full jitter.
func ExponentialBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}
		d := base << (attempt - 1)
		if d > max || d <= 0 {
			d = max
		}
		return time.Duration(rand.Int64N(int64(d) + 1))
	}
}

// WithRetry sets how many attempts a one-shot Execute makes on connection-level
// failures and the backoff between them. Attempts of 0 or 1 disables retry.
func WithRetry(attempts int, backoff BackoffFunc) Option {
	return func(c *Client) {
		c.attempts = attempts
		c.backoff = backoff
	}
}

// isRetryable reports whether err is a transient connection-level failure worth
// retrying. Auth and command-validation errors are never retried.
func isRetryable(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, rcon.ErrAuthFailed),
		errors.Is(err, rcon.ErrCommandEmpty),
		errors.Is(err, rcon.ErrCommandTooLong):
		return false
	case errors.Is(err, io.EOF):
		return true
	}
	_, ok := errors.AsType[net.Error](err)
	return ok
}
