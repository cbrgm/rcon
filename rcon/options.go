package rcon

import "time"

// Default option values used when no override is supplied.
const (
	// DefaultDialTimeout bounds establishing the TCP connection in Dial.
	DefaultDialTimeout = 5 * time.Second
	// DefaultDeadline bounds each individual read and write on the connection.
	DefaultDeadline = 5 * time.Second
	// DefaultMaxCommandLen is the default upper bound on a command's length, a
	// safe margin under the RCON wire request cap. Zero means unlimited.
	DefaultMaxCommandLen = 1000
)

type settings struct {
	dialTimeout   time.Duration
	deadline      time.Duration
	maxCommandLen int
	multiPacket   bool
}

// Option configures a Conn created by Dial or Open.
type Option func(*settings)

// WithDialTimeout sets the timeout for establishing the TCP connection. It has
// no effect on Open, which receives an already-connected socket.
func WithDialTimeout(d time.Duration) Option {
	return func(s *settings) { s.dialTimeout = d }
}

// WithDeadline sets the per-operation read/write deadline on the connection. A
// value of 0 (or negative) disables the per-operation deadline, leaving each
// operation bounded only by any deadline carried on the caller's context.
func WithDeadline(d time.Duration) Option {
	return func(s *settings) { s.deadline = d }
}

// WithMaxCommandLen caps the length of a command passed to Execute. Zero means
// no limit.
func WithMaxCommandLen(n int) Option {
	return func(s *settings) { s.maxCommandLen = n }
}

// WithSinglePacket disables multi-packet response reassembly, reading exactly
// one response packet per command. Use it only for servers that mishandle the
// empty-response terminator sentinel.
func WithSinglePacket() Option {
	return func(s *settings) { s.multiPacket = false }
}

func newSettings(opts []Option) settings {
	s := settings{
		dialTimeout:   DefaultDialTimeout,
		deadline:      DefaultDeadline,
		maxCommandLen: DefaultMaxCommandLen,
		multiPacket:   true,
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}
