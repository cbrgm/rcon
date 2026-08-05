package rconserver

import (
	"errors"
	"net"
	"testing"

	"github.com/cbrgm/rcon/rcon"
)

// drives authenticate over a pipe: sends an AUTH packet with pw, returns the
// server's result and the packets the client received.
func runAuth(t *testing.T, s *Server, pw string) (bool, error, []rcon.Packet) {
	t.Helper()
	client, server := net.Pipe()
	type res struct {
		ok  bool
		err error
	}
	ch := make(chan res, 1)
	go func() {
		ok, err := s.authenticate(server)
		_ = server.Close()
		ch <- res{ok, err}
	}()
	_, _ = (rcon.Packet{ID: 1, Type: rcon.TypeAuth, Body: pw}).WriteTo(client)
	var got []rcon.Packet
	for {
		var p rcon.Packet
		if _, err := p.ReadFrom(client); err != nil {
			break
		}
		got = append(got, p)
	}
	r := <-ch
	return r.ok, r.err, got
}

func TestAuthenticateSuccess(t *testing.T) {
	ok, err, pkts := runAuth(t, &Server{Password: "secret"}, "secret")
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	last := pkts[len(pkts)-1]
	if last.Type != rcon.TypeAuthResponse || last.ID != 1 {
		t.Fatalf("auth response = %+v, want type AUTH_RESPONSE id 1", last)
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	ok, err, pkts := runAuth(t, &Server{Password: "secret"}, "nope")
	if ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	last := pkts[len(pkts)-1]
	if last.Type != rcon.TypeAuthResponse || last.ID != -1 {
		t.Fatalf("auth response = %+v, want id -1 on failure", last)
	}
}

func TestAuthenticatorOverridesPassword(t *testing.T) {
	s := &Server{Password: "ignored", Authenticator: func(pw string) bool { return pw == "viaFunc" }}
	if ok, _, _ := runAuth(t, s, "viaFunc"); !ok {
		t.Fatal("Authenticator should have accepted")
	}
	if ok, _, _ := runAuth(t, s, "ignored"); ok {
		t.Fatal("Authenticator should override Password")
	}
}

func TestAuthenticateRejectsNonAuthFirstPacket(t *testing.T) {
	client, server := net.Pipe()
	go func() {
		_, _ = (rcon.Packet{ID: 1, Type: rcon.TypeExecCommand, Body: "list"}).WriteTo(client)
		_ = client.Close()
	}()
	_, err := (&Server{Password: "x"}).authenticate(server)
	if !errors.Is(err, errNotAuth) {
		t.Fatalf("err = %v, want errNotAuth", err)
	}
}

func TestValidate(t *testing.T) {
	h := HandlerFunc(func(ResponseWriter, *Request) {})
	if err := (&Server{Handler: h, Password: "x"}).validate(); err != nil {
		t.Fatalf("valid server rejected: %v", err)
	}
	if err := (&Server{Password: "x"}).validate(); err == nil {
		t.Fatal("nil Handler should be rejected")
	}
	if err := (&Server{Handler: h}).validate(); err == nil {
		t.Fatal("no password and no authenticator should be rejected")
	}
}
