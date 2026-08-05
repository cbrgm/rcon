package rcon_test

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rcon"
)

func TestDialAndClose(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	c, err := rcon.Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c.RemoteAddr() == nil {
		t.Fatal("RemoteAddr is nil")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDialWrongPassword(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	_, err := rcon.Dial(t.Context(), srv.Addr(), "nope")
	if !errors.Is(err, rcon.ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

func TestOpenWrapsDialedConn(t *testing.T) {
	// Open must work over any net.Conn the caller supplies; drive it over a real
	// TCP connection the test dialed itself.
	srv := fakercon.Start(t, "pw")
	nc, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	c, err := rcon.Open(t.Context(), nc, "pw")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = c.Close()
}

func TestExecuteSimple(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "There are 2 players online")
	c, err := rcon.Dial(t.Context(), srv.Addr(), "secret", rcon.WithSinglePacket())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got, err := c.Execute(t.Context(), "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "There are 2 players online" {
		t.Fatalf("got %q", got)
	}
}

func TestExecuteEmptyCommand(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	c, _ := rcon.Dial(t.Context(), srv.Addr(), "secret", rcon.WithSinglePacket())
	defer c.Close()
	_, err := c.Execute(t.Context(), "")
	if !errors.Is(err, rcon.ErrCommandEmpty) {
		t.Fatalf("err = %v, want ErrCommandEmpty", err)
	}
}

func TestExecuteMultiPacket(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	const total = 3*rcon.MaxPayloadSize + 123 // spans 4 packets
	srv.HandleLong("bigcmd", total)
	c, err := rcon.Dial(t.Context(), srv.Addr(), "secret") // multiPacket on by default
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got, err := c.Execute(t.Context(), "bigcmd")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != total {
		t.Fatalf("len(got) = %d, want %d", len(got), total)
	}
	if strings.Trim(got, "A") != "" {
		t.Fatal("body contained unexpected bytes")
	}
}

// TestExecuteMultiPacketSourceMirrorSequential proves the trailing-mirror
// drain (FIX 1): a real Source server appends a junk "mirror" packet after the
// multi-packet terminator echo. Without draining it, that stale packet desyncs
// the connection and the SECOND Execute fails with ErrResponseMismatch. Both
// Execute calls run on ONE Conn and must return the full correct body.
func TestExecuteMultiPacketSourceMirrorSequential(t *testing.T) {
	const total = 2*rcon.MaxPayloadSize + 7 // spans multiple packets
	addr := startSourceMirrorServer(t, "secret", "bigcmd", total)

	c, err := rcon.Dial(t.Context(), addr, "secret") // multiPacket on by default
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i := 1; i <= 2; i++ {
		got, err := c.Execute(t.Context(), "bigcmd")
		if err != nil {
			t.Fatalf("Execute #%d: %v", i, err)
		}
		if len(got) != total {
			t.Fatalf("Execute #%d: len(got) = %d, want %d", i, len(got), total)
		}
		if strings.Trim(got, "A") != "" {
			t.Fatalf("Execute #%d: body contained unexpected bytes", i)
		}
	}
}

func TestExecuteAfterCloseReturnsErrClosed(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	c, err := rcon.Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err = c.Execute(t.Context(), "list")
	if !errors.Is(err, rcon.ErrClosed) {
		t.Fatalf("err = %v, want ErrClosed", err)
	}
}

// serveSilentAfterAuth authenticates a single client over conn, then reads and
// discards every subsequent packet without ever replying, so a client Execute
// blocks on its response read until the connection is closed. It is the "silent
// peer" the deadlock regression test needs.
func serveSilentAfterAuth(conn net.Conn, password string) {
	var auth rcon.Packet
	if _, err := auth.ReadFrom(conn); err != nil {
		return
	}
	_, _ = rcon.Packet{ID: auth.ID, Type: rcon.TypeResponseValue}.WriteTo(conn)
	id := auth.ID
	if auth.Body != password {
		id = -1
	}
	_, _ = rcon.Packet{ID: id, Type: rcon.TypeAuthResponse}.WriteTo(conn)
	for {
		var junk rcon.Packet
		if _, err := junk.ReadFrom(conn); err != nil {
			return // connection closed
		}
	}
}

// TestCloseAbortsBlockedExecute is the regression test for the deadlock the
// prior fix introduced: with WithDeadline(0) and a deadline-less context, an
// Execute against a silent peer blocks forever on the response read. Because
// Execute held the mutex across that blocking read while the old Close tried to
// acquire it, a concurrent Close deadlocked. Close is now lock-free, so it must
// close the socket, unblock Execute, and both must return promptly.
func TestCloseAbortsBlockedExecute(t *testing.T) {
	client, server := net.Pipe()
	go serveSilentAfterAuth(server, "pw")

	c, err := rcon.Open(t.Context(), client, "pw", rcon.WithDeadline(0))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	execErr := make(chan error, 1)
	go func() {
		_, err := c.Execute(t.Context(), "list")
		execErr <- err
	}()

	// Give Execute time to send its request and block on the response read.
	time.Sleep(50 * time.Millisecond)

	closeErr := make(chan error, 1)
	go func() { closeErr <- c.Close() }()

	select {
	case <-closeErr:
		// Close returned promptly, as required.
	case <-time.After(time.Second):
		t.Fatal("Close did not return within 1s: the lock-free-Close deadlock has reappeared")
	}

	select {
	case err := <-execErr:
		if err == nil {
			t.Fatal("blocked Execute returned nil error after Close, want a read/closed error")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Execute did not return within 1s after Close: read was not aborted")
	}
}

func TestExecuteTooLong(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	c, _ := rcon.Dial(t.Context(), srv.Addr(), "secret", rcon.WithSinglePacket(), rcon.WithMaxCommandLen(5))
	defer c.Close()
	_, err := c.Execute(t.Context(), "way too long")
	if !errors.Is(err, rcon.ErrCommandTooLong) {
		t.Fatalf("err = %v, want ErrCommandTooLong", err)
	}
}

// TestExecuteZeroChunkMultiPacket covers a multi-packet response with no real
// chunks: the server sends only the terminator echo, so Execute returns an
// empty body without error.
func TestExecuteZeroChunkMultiPacket(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.HandleLong("empty", 0)
	c, err := rcon.Dial(t.Context(), srv.Addr(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.Execute(t.Context(), "empty")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want empty", out)
	}
}

// TestExecuteResponseMismatch drives Execute against a server that answers with
// a packet whose ID matches neither the request nor the -1 tolerance, so the
// mismatch is reported instead of being silently accepted.
func TestExecuteResponseMismatch(t *testing.T) {
	client, server := net.Pipe()
	go serveMismatch(server, "secret")

	c, err := rcon.Open(t.Context(), client, "secret", rcon.WithSinglePacket())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	_, err = c.Execute(t.Context(), "list")
	if !errors.Is(err, rcon.ErrResponseMismatch) {
		t.Fatalf("err = %v, want ErrResponseMismatch", err)
	}
}

// serveMismatch performs the auth handshake, then answers the first command
// with a response packet bearing a bogus ID.
func serveMismatch(conn net.Conn, password string) {
	defer conn.Close()
	var auth rcon.Packet
	if _, err := auth.ReadFrom(conn); err != nil {
		return
	}
	_ = password // the mismatch case does not depend on the password
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeAuthResponse}).WriteTo(conn)

	var cmd rcon.Packet
	if _, err := cmd.ReadFrom(conn); err != nil {
		return
	}
	_, _ = (rcon.Packet{ID: cmd.ID + 4242, Type: rcon.TypeResponseValue, Body: "nope"}).WriteTo(conn)
}

// startSourceMirrorServer starts a raw listener that imitates a real Source
// (SRCDS) server: it answers the client's multi-packet terminator with the
// empty echo AND a trailing junk "mirror" packet (body 0x00 0x01 0x00 0x00). A
// compliant server (ours) never emits that junk, so this deliberately
// non-standard behavior lives here in the client test rather than in the fake
// server, which now runs on rconserver. It exists to prove the client's mirror
// drain tolerates it.
func startSourceMirrorServer(t *testing.T, password, cmd string, total int) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSourceMirror(conn, password, cmd, total)
		}
	}()
	return ln.Addr().String()
}

func serveSourceMirror(conn net.Conn, password, cmd string, total int) {
	defer conn.Close()
	authed := false
	for {
		var req rcon.Packet
		if _, err := req.ReadFrom(conn); err != nil {
			return
		}
		switch req.Type {
		case rcon.TypeAuth:
			_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
			id := req.ID
			if req.Body != password {
				id = -1
			} else {
				authed = true
			}
			_, _ = (rcon.Packet{ID: id, Type: rcon.TypeAuthResponse}).WriteTo(conn)
		case rcon.TypeExecCommand:
			if !authed {
				return
			}
			if req.Body == cmd {
				body := strings.Repeat("A", total)
				for len(body) > 0 {
					n := min(len(body), rcon.MaxPayloadSize)
					_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue, Body: body[:n]}).WriteTo(conn)
					body = body[n:]
				}
			} else {
				_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
			}
		case rcon.TypeResponseValue:
			// Echo the terminator AND the trailing junk mirror in one burst, so
			// both land in a single TCP segment (matching real Source servers and
			// keeping the client's zero-wait drain deterministic).
			var burst bytes.Buffer
			_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue}).WriteTo(&burst)
			_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue, Body: "\x00\x01\x00\x00"}).WriteTo(&burst)
			_, _ = conn.Write(burst.Bytes())
		}
	}
}
