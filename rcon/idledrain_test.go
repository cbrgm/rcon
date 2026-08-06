package rcon_test

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// startHybridServer starts an RCON server that mimics Project Zomboid: it
// authenticates any password, never echoes the multi-packet terminator
// sentinel, and answers each command by writing every body in parts as its own
// packet (so a reply can span several packets), then goes quiet and keeps the
// connection open. That combination breaks both multi-packet mode (no working
// terminator) and single-packet mode (a split reply truncates).
func startHybridServer(t *testing.T, parts ...string) string {
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
			go serveHybrid(conn, parts)
		}
	}()
	return ln.Addr().String()
}

func serveHybrid(conn net.Conn, parts []string) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)

	var auth rcon.Packet
	if _, err := auth.ReadFrom(br); err != nil {
		return
	}
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeAuthResponse}).WriteTo(conn)

	for {
		var cmd rcon.Packet
		if _, err := cmd.ReadFrom(br); err != nil {
			return
		}
		// Reply with one packet per part, no terminator sentinel, then stay quiet.
		for _, part := range parts {
			_, _ = (rcon.Packet{ID: cmd.ID, Type: rcon.TypeResponseValue, Body: part}).WriteTo(conn)
		}
	}
}

func TestReadUntilIdleReassemblesSplitReply(t *testing.T) {
	addr := startHybridServer(t, "chunk-A|", "chunk-B|", "chunk-C")
	ctx := t.Context()

	conn, err := rcon.Dial(ctx, addr, "pw", rcon.WithReadUntilIdle(40*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	const want = "chunk-A|chunk-B|chunk-C"

	// Two commands in a row: the split reply must be reassembled, and the stream
	// must stay in sync so the second command also returns the full reply.
	for i := range 2 {
		out, err := conn.Execute(ctx, "help")
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		if out != want {
			t.Fatalf("command %d: out = %q, want %q", i, out, want)
		}
	}
}
