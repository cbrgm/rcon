package rcon

import (
	"errors"
	"net"
	"testing"
)

// These two tests exercise the unexported authenticate function directly, so
// they must stay in the internal (package rcon) test binary. They drive the
// handshake over a net.Pipe with an inline responder rather than
// internal/fakercon, because fakercon imports rcon: an internal test file
// importing a package that imports back into rcon is an import cycle. The
// rest of this package's black-box tests live in conn_external_test.go
// (package rcon_test) where importing fakercon is safe.
func TestAuthenticateSuccess(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	go respondToAuth(server, "secret")

	var id int32
	next := func() int32 { id++; return id }
	if err := authenticate(client, "secret", next); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	go respondToAuth(server, "secret")

	var id int32
	next := func() int32 { id++; return id }
	err := authenticate(client, "wrong", next)
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

// respondToAuth reads a single TypeAuth packet from conn and replies with the
// standard auth handshake: an empty TypeResponseValue sentinel followed by
// the TypeAuthResponse.
func respondToAuth(conn net.Conn, password string) {
	defer conn.Close()
	var req Packet
	if _, err := req.ReadFrom(conn); err != nil {
		return
	}
	_, _ = (Packet{ID: req.ID, Type: TypeResponseValue}).WriteTo(conn)
	id := req.ID
	if req.Body != password {
		id = -1
	}
	_, _ = (Packet{ID: id, Type: TypeAuthResponse}).WriteTo(conn)
}
