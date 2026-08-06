package rconclient_test

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

// startHybridServer answers each command with two packets and no terminator
// sentinel, then goes quiet, mimicking a Project Zomboid-style split reply.
func startHybridServer(t *testing.T) string {
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
			go func() {
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
					_, _ = (rcon.Packet{ID: cmd.ID, Type: rcon.TypeResponseValue, Body: "left-"}).WriteTo(conn)
					_, _ = (rcon.Packet{ID: cmd.ID, Type: rcon.TypeResponseValue, Body: "right"}).WriteTo(conn)
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func TestWithReadUntilIdleReassembles(t *testing.T) {
	addr := startHybridServer(t)
	ctx := t.Context()

	client := rconclient.New(rconclient.WithReadUntilIdle(40 * time.Millisecond))
	session, err := client.Dial(ctx, addr, "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Two commands over one session: reassembled reply, and stream stays in sync.
	for i := range 2 {
		out, err := session.Execute(ctx, "help")
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		if out != "left-right" {
			t.Fatalf("command %d: out = %q, want %q", i, out, "left-right")
		}
	}
}
