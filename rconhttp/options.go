package rconhttp

import (
	"log/slog"
	"time"

	"github.com/cbrgm/rcon/rconclient"
)

// DefaultIdleTimeout is how long a cached backend session may sit idle before it
// is closed and evicted.
const DefaultIdleTimeout = 5 * time.Minute

type config struct {
	resolver Resolver
	client   *rconclient.Client
	idle     time.Duration
	logger   *slog.Logger
}

// Option configures a Handler.
type Option func(*config)

// WithResolver replaces the fixed backend with a per-request Resolver.
func WithResolver(res Resolver) Option {
	return func(c *config) { c.resolver = res }
}

// WithClient supplies a preconfigured rconclient.Client (timeouts, retry,
// logger). It defaults to rconclient.New().
func WithClient(cl *rconclient.Client) Option {
	return func(c *config) { c.client = cl }
}

// WithIdleTimeout sets how long a cached session may sit idle before eviction.
func WithIdleTimeout(d time.Duration) Option {
	return func(c *config) { c.idle = d }
}

// WithLogger sets the handler's structured logger. It defaults to a no-op logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}
