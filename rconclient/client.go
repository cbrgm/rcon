package rconclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// BackoffFunc computes the delay before retry attempt n (1-based).
type BackoffFunc func(attempt int) time.Duration

// Client is a reusable, concurrency-safe high-level RCON client. Construct it
// with New; the zero value is not usable. A Client is safe for use by multiple
// goroutines, mirroring *http.Client.
type Client struct {
	timeout      time.Duration
	dialTimeout  time.Duration
	attempts     int
	backoff      BackoffFunc
	dialer       DialFunc
	singlePacket bool
	idleDrain    bool
	idleWindow   time.Duration
	logger       *slog.Logger
}

// DefaultClient is used by the package-level Execute helper.
var DefaultClient = New()

// New returns a Client configured with opts.
func New(opts ...Option) *Client {
	c := &Client{
		timeout:     DefaultTimeout,
		dialTimeout: DefaultDialTimeout,
		attempts:    1,
		backoff:     nil,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Execute dials address, authenticates with password, runs command, and closes
// the connection. It is the one-shot path; for many commands use Dial + Session.
// The whole call, including any retries, is bounded by the Client's timeout
// (see WithTimeout). Connection-level failures (dial error, io.EOF, net.Error)
// are retried per WithRetry; rcon.ErrAuthFailed, ErrCommandEmpty, and
// ErrCommandTooLong are never retried.
func (c *Client) Execute(ctx context.Context, address, password, command string) (string, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	attempts := c.attempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		out, err := c.executeOnce(ctx, address, password, command)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == attempts {
			break
		}
		if c.backoff != nil {
			select {
			case <-time.After(c.backoff(attempt)):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		c.logger.Debug("retrying rcon execute", "attempt", attempt+1, "address", address, "err", err)
	}
	return "", lastErr
}

func (c *Client) executeOnce(ctx context.Context, address, password, command string) (string, error) {
	conn, err := c.dial(ctx, address, password)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
	return conn.Execute(ctx, command)
}

func (c *Client) dial(ctx context.Context, address, password string) (*rcon.Conn, error) {
	var opts []rcon.Option
	switch {
	case c.idleDrain:
		opts = append(opts, rcon.WithReadUntilIdle(c.idleWindow))
	case c.singlePacket:
		opts = append(opts, rcon.WithSinglePacket())
	}
	if c.dialer != nil {
		nc, err := c.dialer(ctx, address)
		if err != nil {
			return nil, err
		}
		if nc == nil {
			return nil, errors.New("rconclient: dialer returned a nil connection with no error")
		}
		conn, err := rcon.Open(ctx, nc, password, opts...)
		if err != nil {
			_ = nc.Close() // rcon.Open does not own nc; close it on failure.
			return nil, err
		}
		return conn, nil
	}
	return rcon.Dial(ctx, address, password, append([]rcon.Option{rcon.WithDialTimeout(c.dialTimeout)}, opts...)...)
}

// Execute runs a single command using DefaultClient.
func Execute(ctx context.Context, address, password, command string) (string, error) {
	return DefaultClient.Execute(ctx, address, password, command)
}
