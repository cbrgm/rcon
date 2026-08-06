package rconclient_test

import (
	"bufio"
	"net"
	"testing"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

// startSinglePacketServer starts a minimal RCON server that authenticates any
// password and answers each command with exactly one response packet. It never
// echoes the empty multi-packet terminator sentinel and hangs up right after the
// reply, mimicking single-packet servers like Project Zomboid.
func startSinglePacketServer(t *testing.T) string {
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
			go serveSinglePacket(conn)
		}
	}()
	return ln.Addr().String()
}

func serveSinglePacket(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)

	var auth rcon.Packet
	if _, err := auth.ReadFrom(br); err != nil {
		return
	}
	// Empty RESPONSE_VALUE then AUTH_RESPONSE echoing the id => auth success.
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeAuthResponse}).WriteTo(conn)

	var cmd rcon.Packet
	if _, err := cmd.ReadFrom(br); err != nil {
		return
	}
	// One reply, then hang up. Any trailing terminator sentinel the client sent
	// is deliberately left unread.
	_, _ = (rcon.Packet{ID: cmd.ID, Type: rcon.TypeResponseValue, Body: "pong"}).WriteTo(conn)
}

func TestWithSinglePacketReadsOneReply(t *testing.T) {
	addr := startSinglePacketServer(t)
	ctx := t.Context()

	// The default (multi-packet) client waits for a terminator echo this server
	// never sends; the hang-up surfaces as an error.
	if _, err := rconclient.New().Execute(ctx, addr, "pw", "ping"); err == nil {
		t.Fatal("multi-packet client: want error against a single-packet server, got nil")
	}

	// WithSinglePacket reads exactly one reply and returns it.
	out, err := rconclient.New(rconclient.WithSinglePacket()).Execute(ctx, addr, "pw", "ping")
	if err != nil {
		t.Fatalf("single-packet client: %v", err)
	}
	if out != "pong" {
		t.Fatalf("out = %q, want %q", out, "pong")
	}
}
