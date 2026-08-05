package rconclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

// TestSessionExecuteHonorsTimeout drives a Session against a peer that
// authenticates then stalls, so the only thing that can end the command is the
// Client's WithTimeout. Without that timeout applied, the command would block
// until the core's 5s per-op deadline; the 2s guard fails in that case.
func TestSessionExecuteHonorsTimeout(t *testing.T) {
	client, server := net.Pipe()
	go stallAfterAuth(server, "secret")

	c := rconclient.New(
		rconclient.WithTimeout(200*time.Millisecond),
		rconclient.WithDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return client, nil
		}),
	)
	s, err := c.Dial(t.Context(), "pipe", "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer s.Close()

	start := time.Now()
	_, err = s.Execute(t.Context(), "hang")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Session.Execute took %v; client WithTimeout was not applied", elapsed)
	}
}

// stallAfterAuth performs the auth handshake, reads the command packets, then
// blocks on a final read that only returns once the client closes the pipe.
func stallAfterAuth(conn net.Conn, password string) {
	defer conn.Close()
	var auth rcon.Packet
	if _, err := auth.ReadFrom(conn); err != nil {
		return
	}
	_, _ = (rcon.Packet{ID: auth.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
	id := auth.ID
	if auth.Body != password {
		id = -1
	}
	_, _ = (rcon.Packet{ID: id, Type: rcon.TypeAuthResponse}).WriteTo(conn)

	// Read the exec command and the trailing sentinel, then stall. The final
	// read unblocks (with an error) when the client closes its end.
	var p rcon.Packet
	_, _ = p.ReadFrom(conn)
	_, _ = p.ReadFrom(conn)
	_, _ = p.ReadFrom(conn)
}
