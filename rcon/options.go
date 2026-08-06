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
	// DefaultIdleWindow is the quiet period that marks a response complete in
	// read-until-idle mode (see WithReadUntilIdle).
	DefaultIdleWindow = 100 * time.Millisecond
)

// readMode selects how Execute reads a command's response.
type readMode int

const (
	// readMulti reassembles multi-packet responses via the empty-response
	// terminator sentinel. It is the default and is correct for Source-engine
	// servers.
	readMulti readMode = iota
	// readSingle reads exactly one response packet per command.
	readSingle
	// readIdle reads response packets until the connection goes quiet.
	readIdle
)

type settings struct {
	dialTimeout   time.Duration
	deadline      time.Duration
	maxCommandLen int
	mode          readMode
	idleWindow    time.Duration
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
// empty-response terminator sentinel and never split a response across packets.
func WithSinglePacket() Option {
	return func(s *settings) { s.mode = readSingle }
}

// WithReadUntilIdle reads response packets until no further data arrives within
// window, concatenating their bodies, instead of using the terminator sentinel.
// It suits servers that mishandle that sentinel yet still split large responses
// across packets, such as Project Zomboid. A window of 0 or less keeps
// DefaultIdleWindow.
//
// It assumes a response's packets arrive within one window of each other, which
// holds on local and LAN links where a server flushes a response as one burst.
// A slow link that stalls mid-response for longer than window could end the read
// early, so prefer WithReadUntilIdle over the default only for servers that need
// it, and size window to the link.
func WithReadUntilIdle(window time.Duration) Option {
	return func(s *settings) {
		s.mode = readIdle
		if window > 0 {
			s.idleWindow = window
		}
	}
}

func newSettings(opts []Option) settings {
	s := settings{
		dialTimeout:   DefaultDialTimeout,
		deadline:      DefaultDeadline,
		maxCommandLen: DefaultMaxCommandLen,
		mode:          readMulti,
		idleWindow:    DefaultIdleWindow,
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}
