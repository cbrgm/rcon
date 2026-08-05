// Package rconclient is a high-level RCON client built on github.com/cbrgm/rcon.
//
// It mirrors the shape of net/http: a DefaultClient, package-level helper
// functions that delegate to it, and an instantiable Client that is safe for
// concurrent use. For repeated commands to one server, use a Session.
package rconclient

import (
	"context"
	"log/slog"
	"net"
	"time"
)

// Default option values.
const (
	// DefaultTimeout bounds a single command round-trip.
	DefaultTimeout = 10 * time.Second
	// DefaultDialTimeout bounds establishing the connection.
	DefaultDialTimeout = 5 * time.Second
)

// Option configures a Client.
type Option func(*Client)

// WithTimeout sets the overall per-command deadline.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithDialTimeout sets the connection dial timeout.
func WithDialTimeout(d time.Duration) Option { return func(c *Client) { c.dialTimeout = d } }

// DialFunc establishes a raw connection to address. It lets the client run over
// TLS, an SSH tunnel, a proxy, or any custom transport instead of plain TCP.
type DialFunc func(ctx context.Context, address string) (net.Conn, error)

// WithDialer makes the Client establish connections with d and wrap them via
// rcon.Open, instead of dialing plain TCP itself. When a dialer is set,
// WithDialTimeout does not apply, since d owns connection setup. The Client's
// WithTimeout still bounds the whole call.
//
// d must return a non-nil connection when it returns a nil error. A dial error
// is retried by WithRetry only when it satisfies net.Error, so wrap transport
// errors accordingly if you want them retried.
func WithDialer(d DialFunc) Option { return func(c *Client) { c.dialer = d } }

// WithLogger sets the structured logger. The default logger discards output.
func WithLogger(l *slog.Logger) Option { return func(c *Client) { c.logger = l } }
