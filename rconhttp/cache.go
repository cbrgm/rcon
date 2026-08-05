package rconhttp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

// sessionCache holds one reused, auto-reconnecting session per backend, keyed by
// address and password.
type sessionCache struct {
	client *rconclient.Client
	idle   time.Duration
	logger *slog.Logger

	mu      sync.Mutex
	entries map[string]*cacheEntry
	done    chan struct{}

	closeOnce sync.Once
}

// cacheEntry is one backend's session. session and its use are guarded by mu;
// lastUsed is guarded by the parent sessionCache.mu.
type cacheEntry struct {
	mu       sync.Mutex
	backend  Backend
	session  *rconclient.Session
	lastUsed time.Time
}

func newCache(client *rconclient.Client, idle time.Duration, logger *slog.Logger) *sessionCache {
	return &sessionCache{
		client:  client,
		idle:    idle,
		logger:  logger,
		entries: make(map[string]*cacheEntry),
		done:    make(chan struct{}),
	}
}

func cacheKey(b Backend) string { return b.Addr + "\x00" + b.Password }

// get returns the entry for b, creating it if needed, and marks it used. It
// holds the cache lock only briefly; the returned entry is used under its own
// mutex. Marking lastUsed here keeps the reaper from evicting an entry a request
// just picked up.
func (c *sessionCache) get(b Backend) *cacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(b)
	e := c.entries[key]
	if e == nil {
		e = &cacheEntry{backend: b}
		c.entries[key] = e
	}
	e.lastUsed = time.Now()
	return e
}

// execute runs cmd against b over the backend's reused session, dialing lazily
// and dropping the session on a connection-level error so the next call redials.
func (c *sessionCache) execute(ctx context.Context, b Backend, cmd string) (string, error) {
	e := c.get(b)
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session == nil {
		s, err := c.client.Dial(ctx, b.Addr, b.Password)
		if err != nil {
			c.logger.Warn("rcon backend dial failed", "addr", b.Addr, "err", err)
			return "", err
		}
		e.session = s
	}

	out, err := e.session.Execute(ctx, cmd)
	if err != nil && isConnError(err) {
		c.logger.Debug("dropping unusable rcon session", "addr", b.Addr, "err", err)
		_ = e.session.Close()
		e.session = nil
	}
	return out, err
}

// startReaper runs a background loop that evicts idle entries. It is a no-op
// when idle <= 0.
func (c *sessionCache) startReaper() {
	if c.idle <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(c.idle)
		defer t.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-t.C:
				c.reap()
			}
		}
	}()
}

// reap closes and removes every entry idle longer than the timeout.
func (c *sessionCache) reap() {
	cutoff := time.Now().Add(-c.idle)
	c.mu.Lock()
	var stale []*cacheEntry
	for key, e := range c.entries {
		if e.lastUsed.Before(cutoff) {
			stale = append(stale, e)
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()

	for _, e := range stale {
		e.mu.Lock()
		if e.session != nil {
			c.logger.Debug("evicting idle rcon session", "addr", e.backend.Addr)
			_ = e.session.Close()
			e.session = nil
		}
		e.mu.Unlock()
	}
}

// close stops the reaper once and closes all sessions.
func (c *sessionCache) close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return c.closeAll()
}

// closeAll closes every cached session and empties the cache.
func (c *sessionCache) closeAll() error {
	c.mu.Lock()
	entries := c.entries
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()

	for _, e := range entries {
		e.mu.Lock()
		if e.session != nil {
			_ = e.session.Close()
			e.session = nil
		}
		e.mu.Unlock()
	}
	return nil
}

// isConnError reports whether err is a connection-level failure that should
// invalidate a cached session.
func isConnError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, rcon.ErrClosed) || errors.Is(err, rcon.ErrResponseMismatch) {
		return true
	}
	_, ok := errors.AsType[net.Error](err)
	return ok
}
