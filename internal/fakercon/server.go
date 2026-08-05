// Package fakercon implements a minimal in-process Source RCON server for
// tests. It is built on rconserver, so every test that uses it also exercises
// the module's own server implementation. It is internal so it never leaks into
// the public API.
package fakercon

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/cbrgm/rcon/rconserver"
)

// errDrop is the panic value used to simulate a mid-session connection drop.
// rconserver's per-command panic recovery closes the connection when a handler
// panics, which the client observes as EOF.
var errDrop = errors.New("fakercon: simulated connection drop")

// Server is a fake RCON server backed by rconserver.
type Server struct {
	srv  *rconserver.Server
	addr string

	mu       sync.Mutex
	replies  map[string]string // command -> single reply
	longCmds map[string]int    // command -> total body length to synthesize
	dropOnce bool              // drop the next served command mid-session
}

// Start listens on 127.0.0.1:0 and serves connections until Close. If tb is
// non-nil, Start registers a cleanup that closes the server when the test
// completes.
func Start(tb testing.TB, password string) *Server {
	if tb != nil {
		tb.Helper()
	}
	s := &Server{
		replies:  map[string]string{},
		longCmds: map[string]int{},
	}
	s.srv = &rconserver.Server{Password: password, Handler: s}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if tb != nil {
			tb.Fatalf("listen: %v", err)
		}
		return nil
	}
	s.addr = ln.Addr().String()
	go func() { _ = s.srv.Serve(ln) }()

	if tb != nil {
		tb.Cleanup(func() { _ = s.Close() })
	}
	return s
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string { return s.addr }

// Handle configures a single-packet reply for cmd.
func (s *Server) Handle(cmd, reply string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replies[cmd] = reply
}

// HandleLong configures cmd to synthesize a reply of totalLen bytes, which
// rconserver splits across as many RESPONSE_VALUE packets as needed.
func (s *Server) HandleLong(cmd string, totalLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.longCmds[cmd] = totalLen
}

// DropNext makes the server drop the connection the next time it executes a
// command, without replying, simulating a mid-session connection failure. The
// flag is consumed on use, so a client that reconnects and retries the command
// succeeds. It is one-shot and off by default.
func (s *Server) DropNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropOnce = true
}

// Close stops the server, closing the listener and any live connections.
func (s *Server) Close() error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Close()
}

// ServeRCON implements rconserver.Handler. It dispatches a command from the
// configured maps, or panics with errDrop when a drop is armed so rconserver
// closes the connection.
func (s *Server) ServeRCON(w rconserver.ResponseWriter, r *rconserver.Request) {
	s.mu.Lock()
	if s.dropOnce {
		s.dropOnce = false
		s.mu.Unlock()
		panic(errDrop)
	}
	reply, ok := s.replies[r.Command]
	longLen, isLong := s.longCmds[r.Command]
	s.mu.Unlock()

	switch {
	case isLong:
		_, _ = io.WriteString(w, strings.Repeat("A", longLen))
	case ok:
		_, _ = io.WriteString(w, reply)
	default:
		// No write: rconserver sends a single empty RESPONSE_VALUE, matching the
		// old fake server's reply for an unknown command.
	}
}
