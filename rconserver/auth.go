package rconserver

import (
	"crypto/subtle"
	"errors"
	"io"

	"github.com/cbrgm/rcon/rcon"
)

var errNotAuth = errors.New("rconserver: first packet was not an auth request")

// validate reports whether the server is safe to run.
func (s *Server) validate() error {
	if s.Handler == nil {
		return errors.New("rconserver: nil Handler")
	}
	if s.Authenticator == nil && s.Password == "" {
		return errors.New("rconserver: no Password or Authenticator configured")
	}
	return nil
}

// checkPassword validates a password with the Authenticator when set, otherwise
// a constant-time compare against Password.
func (s *Server) checkPassword(password string) bool {
	if s.Authenticator != nil {
		return s.Authenticator(password)
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(s.Password)) == 1
}

// authenticate performs the RCON auth handshake over rw. The first packet must be
// TypeAuth. It replies with an empty RESPONSE_VALUE sentinel then an
// AUTH_RESPONSE whose id is the request id on success or -1 on failure, and
// reports whether the client authenticated.
func (s *Server) authenticate(rw io.ReadWriter) (bool, error) {
	var req rcon.Packet
	if _, err := req.ReadFrom(rw); err != nil {
		return false, err
	}
	if req.Type != rcon.TypeAuth {
		return false, errNotAuth
	}
	ok := s.checkPassword(req.Body)
	if _, err := (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue}).WriteTo(rw); err != nil {
		return false, err
	}
	id := req.ID
	if !ok {
		id = -1
	}
	if _, err := (rcon.Packet{ID: id, Type: rcon.TypeAuthResponse}).WriteTo(rw); err != nil {
		return false, err
	}
	return ok, nil
}
