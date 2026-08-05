package rconclient

import (
	"context"

	"github.com/cbrgm/rcon/rcon"
)

// Session is a live authenticated connection to one server, for issuing many
// commands. Unlike Client.Execute, it keeps the connection open. A Session is
// not safe for concurrent use; use one per goroutine.
type Session struct {
	client   *Client
	address  string
	password string
	conn     *rcon.Conn
}

// Dial opens a Session to address, authenticated with password.
func (c *Client) Dial(ctx context.Context, address, password string) (*Session, error) {
	conn, err := c.dial(ctx, address, password)
	if err != nil {
		return nil, err
	}
	return &Session{client: c, address: address, password: password, conn: conn}, nil
}

// Execute runs command on the session's connection. On a retryable
// connection-level error (dial error, io.EOF, net.Error), it reconnects once
// and retries the command; auth and command-validation errors are returned
// immediately. The whole call, including the reconnect, is bounded by the
// Client's timeout (see WithTimeout).
func (s *Session) Execute(ctx context.Context, command string) (string, error) {
	if s.client.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.client.timeout)
		defer cancel()
	}

	out, err := s.conn.Execute(ctx, command)
	if err == nil || !isRetryable(err) {
		return out, err
	}
	// Reconnect once and retry.
	_ = s.conn.Close()
	conn, derr := s.client.dial(ctx, s.address, s.password)
	if derr != nil {
		return "", derr
	}
	s.conn = conn
	return s.conn.Execute(ctx, command)
}

// Close closes the underlying connection.
func (s *Session) Close() error { return s.conn.Close() }
