package rcon

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Conn is a single authenticated RCON connection over one TCP socket.
//
// Concurrent calls to Execute are serialized by an internal mutex, so a Conn is
// safe to share, but only one command is ever in flight (RCON has no request
// multiplexing). Close may be called concurrently to abort a blocked Execute:
// it takes no lock and closes the socket directly, unblocking the in-flight
// read.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader
	set  settings

	// closed is an atomic so Close can flip it and tear down the socket without
	// acquiring mu, which Execute holds for the whole blocking round-trip.
	closed atomic.Bool

	mu sync.Mutex
	id int32
}

// Dial connects to address over TCP, authenticates with password, and returns a
// ready Conn. A deadline carried by ctx bounds the dial and the auth handshake;
// cancellation of a deadline-less context is not observed once the dial
// completes.
func Dial(ctx context.Context, address, password string, opts ...Option) (*Conn, error) {
	set := newSettings(opts)
	d := net.Dialer{Timeout: set.dialTimeout}
	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	c, err := open(ctx, netConn, password, set)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	return c, nil
}

// Open wraps an already-established net.Conn (a custom dialer, a TLS or SSH
// tunnel, or net.Pipe in tests), authenticates with password, and returns a
// ready Conn. A deadline carried by ctx bounds the auth handshake; cancellation
// of a deadline-less context is not observed.
func Open(ctx context.Context, netConn net.Conn, password string, opts ...Option) (*Conn, error) {
	return open(ctx, netConn, password, newSettings(opts))
}

func open(ctx context.Context, netConn net.Conn, password string, set settings) (*Conn, error) {
	c := &Conn{conn: netConn, br: bufio.NewReader(netConn), set: set}
	if err := applyContextDeadline(ctx, netConn, set.deadline); err != nil {
		return nil, err
	}
	if err := authenticate(rw{c.br, c.conn}, password, c.nextID); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Conn) nextID() int32 {
	c.id++
	if c.id <= 0 {
		c.id = 1
	}
	return c.id
}

// Close closes the underlying connection. Subsequent Execute calls return
// ErrClosed. It is safe to call concurrently with Execute to abort a blocked
// round-trip: Close takes no lock (Execute holds mu for its entire blocking
// I/O), it just marks the Conn closed and closes the socket, which unblocks
// any in-flight read with an error. The socket close error is returned.
func (c *Conn) Close() error {
	c.closed.Store(true)
	return c.conn.Close()
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr { return c.conn.LocalAddr() }

// RemoteAddr returns the server's network address.
func (c *Conn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// rw pairs a buffered reader with the writer half of the same connection so the
// handshake and command round-trips read through the buffer but write directly.
type rw struct {
	r *bufio.Reader
	w net.Conn
}

func (x rw) Read(p []byte) (int, error)  { return x.r.Read(p) }
func (x rw) Write(p []byte) (int, error) { return x.w.Write(p) }

// Execute sends command to the server and returns the response body. Concurrent
// calls are serialized. A deadline carried by ctx bounds the round-trip;
// cancellation of a deadline-less context is not observed mid-round-trip.
// Execute returns ErrClosed if the Conn has been closed.
func (c *Conn) Execute(ctx context.Context, command string) (string, error) {
	// Reject a closed Conn before contending on mu, which a blocked Execute
	// holds for its whole round-trip; the atomic check needs no lock.
	if c.closed.Load() {
		return "", ErrClosed
	}
	if command == "" {
		return "", ErrCommandEmpty
	}
	if c.set.maxCommandLen > 0 && len(command) > c.set.maxCommandLen {
		return "", ErrCommandTooLong
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := applyContextDeadline(ctx, c.conn, c.set.deadline); err != nil {
		return "", err
	}

	reqID := c.nextID()
	if _, err := (Packet{ID: reqID, Type: TypeExecCommand, Body: command}).WriteTo(c.conn); err != nil {
		return "", err
	}

	switch c.set.mode {
	case readSingle:
		return c.readSinglePacket(reqID)
	case readIdle:
		return c.readIdleDrain(reqID)
	default: // readMulti
		// Send an empty sentinel; the server echoes it after all real chunks.
		sentinelID := c.nextID()
		if _, err := (Packet{ID: sentinelID, Type: TypeResponseValue}).WriteTo(c.conn); err != nil {
			return "", err
		}
		return c.readMultiPacket(reqID, sentinelID)
	}
}

func (c *Conn) readSinglePacket(reqID int32) (string, error) {
	var resp Packet
	if _, err := resp.ReadFrom(c.br); err != nil {
		return "", err
	}
	// Non-Valve servers may reply with ID -1 on a normal response; tolerate it.
	if resp.ID != reqID && resp.ID != -1 {
		return "", ErrResponseMismatch
	}
	return resp.Body, nil
}

// readMultiPacket reads response packets until the empty echo of sentinelID
// arrives, concatenating the bodies of all packets belonging to reqID along
// the way. The sentinel echo marks the end of a (possibly multi-packet)
// response, since the server processes packets in order and only writes it
// after all real response chunks have been sent.
func (c *Conn) readMultiPacket(reqID, sentinelID int32) (string, error) {
	var b strings.Builder
	for {
		var resp Packet
		if _, err := resp.ReadFrom(c.br); err != nil {
			return "", err
		}
		// The empty echo of our sentinel marks the end of the response.
		if resp.ID == sentinelID {
			c.drainSourceMirror()
			return b.String(), nil
		}
		if resp.ID != reqID && resp.ID != -1 {
			return "", ErrResponseMismatch
		}
		b.WriteString(resp.Body)
	}
}

// readIdleDrain reads response packets belonging to reqID until no further data
// arrives within the idle window, concatenating their bodies. It sends no
// terminator sentinel, so it suits servers that mishandle that sentinel yet
// still split large responses across packets (e.g. Project Zomboid). The first
// packet is bounded by the connection's normal deadline (set by Execute); each
// subsequent read is bounded by the idle window, and a read that times out or
// hits EOF marks the response complete. Packets whose id is neither reqID nor
// the tolerated -1 are skipped rather than accumulated.
func (c *Conn) readIdleDrain(reqID int32) (string, error) {
	var b strings.Builder
	first := true
	for {
		if !first {
			if err := c.conn.SetReadDeadline(time.Now().Add(c.set.idleWindow)); err != nil {
				return "", err
			}
		}
		var resp Packet
		if _, err := resp.ReadFrom(c.br); err != nil {
			if first {
				return "", err // no reply within the normal deadline
			}
			if isIdleTimeout(err) || errors.Is(err, io.EOF) {
				return b.String(), nil // quiet: the response is complete
			}
			return "", err
		}
		if resp.ID == reqID || resp.ID == -1 {
			b.WriteString(resp.Body)
		}
		first = false
	}
}

// isIdleTimeout reports whether err is a read deadline expiry, which in
// read-until-idle mode signals the response is complete rather than a failure.
func isIdleTimeout(err error) bool {
	ne, ok := errors.AsType[net.Error](err)
	return ok && ne.Timeout()
}

// drainSourceMirror discards a Source-engine trailing "mirror" packet
// (0x00 0x01 0x00 0x00) that some servers send right after the terminator
// echo, in the same write burst. It only consumes bytes already buffered, so
// it never blocks and never delays servers that send no mirror (e.g. Minecraft).
//
// Tradeoff: a mirror arriving in a separate, later TCP segment (uncommon; the
// mirror is normally in the same burst as the echo) is not drained here. That
// is an accepted limitation to avoid penalizing the common path with a wait.
func (c *Conn) drainSourceMirror() {
	for c.br.Buffered() > 0 {
		var junk Packet
		if _, err := junk.ReadFrom(c.br); err != nil {
			return
		}
	}
}

// applyContextDeadline sets a read/write deadline on conn that is the sooner of
// the per-operation deadline and any deadline carried by ctx. A non-positive
// opDeadline disables the per-operation bound, leaving only the ctx deadline (if
// any); when neither applies, any prior deadline is cleared.
func applyContextDeadline(ctx context.Context, conn net.Conn, opDeadline time.Duration) error {
	var deadline time.Time
	if opDeadline > 0 {
		deadline = time.Now().Add(opDeadline)
	}
	if d, ok := ctx.Deadline(); ok && (deadline.IsZero() || d.Before(deadline)) {
		deadline = d
	}
	return conn.SetDeadline(deadline)
}
