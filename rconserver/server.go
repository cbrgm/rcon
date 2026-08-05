package rconserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// ErrServerClosed is returned by Serve and ListenAndServe after the server is
// stopped by Shutdown or Close.
var ErrServerClosed = errors.New("rconserver: server closed")

// Server is an RCON server. A Server must have a Handler and either a Password or
// an Authenticator.
type Server struct {
	// Addr is the TCP address ListenAndServe listens on.
	Addr string
	// Handler dispatches each authenticated command.
	Handler Handler
	// Password is the shared RCON password, used when Authenticator is nil.
	Password string
	// Authenticator validates a password. When set, it overrides Password.
	Authenticator func(password string) bool
	// ReadTimeout is the deadline applied before every read on a connection,
	// including the initial authentication handshake, so it also bounds how
	// long an unauthenticated client may hold the connection open. Zero means
	// no timeout is applied anywhere on the connection — the same footgun a
	// zero-value net/http.Server has, made explicit here.
	ReadTimeout time.Duration
	// Logger receives server events. A nil Logger discards output.
	Logger *slog.Logger

	// unexported lifecycle state
	mu         sync.Mutex
	conns      map[net.Conn]struct{}
	listeners  map[*net.Listener]struct{}
	inShutdown atomic.Bool
	doneCtx    context.Context
	cancel     context.CancelFunc
	handlerWG  sync.WaitGroup
}

func (s *Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var errHandlerPanic = errors.New("rconserver: handler panicked")

// serveConn authenticates the connection, then serves commands until the client
// disconnects or an error occurs. It always closes conn.
func (s *Server) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	if s.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	}
	authed, err := s.authenticate(conn)
	if err != nil || !authed {
		return
	}

	for {
		if s.ReadTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))
		}
		var req rcon.Packet
		if _, err := req.ReadFrom(conn); err != nil {
			return
		}
		switch req.Type {
		case rcon.TypeExecCommand:
			r := &Request{Command: req.Body, RemoteAddr: conn.RemoteAddr().String(), ctx: s.doneCtx}
			if err := s.dispatch(conn, r, req.ID); err != nil {
				return // handler panicked; drop the connection
			}
		case rcon.TypeResponseValue:
			// Echo the client's multi-packet sentinel.
			_, _ = (rcon.Packet{ID: req.ID, Type: rcon.TypeResponseValue}).WriteTo(conn)
		default:
			// Unknown packet type; ignore for forward compatibility.
		}
	}
}

// dispatch runs the handler with panic recovery and writes the framed response.
// It returns errHandlerPanic if the handler panicked.
func (s *Server) dispatch(conn io.Writer, r *Request, reqID int32) (err error) {
	rw := &responseWriter{}
	s.handlerWG.Add(1)
	defer s.handlerWG.Done()
	defer func() {
		if p := recover(); p != nil {
			s.logger().Error("rconserver: handler panicked", "err", p, "addr", r.RemoteAddr)
			err = errHandlerPanic
		}
	}()
	s.Handler.ServeRCON(rw, r)
	if werr := writeResponse(conn, reqID, rw.buf); werr != nil {
		return werr
	}
	return nil
}

// ListenAndServe listens on s.Addr and serves RCON connections.
func (s *Server) ListenAndServe() error {
	if err := s.validate(); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve accepts connections on ln and serves each in its own goroutine until
// Close or Shutdown, after which it returns ErrServerClosed.
func (s *Server) Serve(ln net.Listener) error {
	if err := s.validate(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.conns == nil {
		s.conns = make(map[net.Conn]struct{})
	}
	if s.listeners == nil {
		s.listeners = make(map[*net.Listener]struct{})
	}
	if s.cancel == nil {
		s.doneCtx, s.cancel = context.WithCancel(context.Background())
	}
	s.mu.Unlock()

	// Close ln when Serve returns, whether via the normal ErrServerClosed
	// path, an Accept error, or the trackListener refusal below. Closing an
	// already-closed listener is harmless. (A validate() failure returns
	// before this point; a direct Serve(ln) caller owns that listener.)
	defer func() { _ = ln.Close() }()

	if !s.trackListener(&ln, true) {
		_ = ln.Close()
		return ErrServerClosed
	}
	defer s.trackListener(&ln, false)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.inShutdown.Load() {
				return ErrServerClosed
			}
			return err
		}
		if !s.trackConn(conn, true) {
			_ = conn.Close()
			return ErrServerClosed
		}
		go func() {
			defer s.trackConn(conn, false)
			s.serveConn(conn)
		}()
	}
}

// Close stops accepting new connections and closes all active ones immediately.
func (s *Server) Close() error {
	s.inShutdown.Store(true)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	var err error
	for lnp := range s.listeners {
		if cerr := (*lnp).Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	for c := range s.conns {
		_ = c.Close()
	}
	return err
}

// Shutdown stops accepting new connections and waits for in-flight handlers to
// finish, bounded by ctx, then closes all remaining connections. It returns
// ctx.Err() if it timed out waiting for handlers. It only waits for handlers
// that have begun executing by the time Shutdown checks; a command accepted
// moments earlier may still race past that check.
func (s *Server) Shutdown(ctx context.Context) error {
	s.inShutdown.Store(true)
	s.mu.Lock()
	for lnp := range s.listeners {
		_ = (*lnp).Close()
	}
	s.mu.Unlock()

	// Signal in-flight handlers that shutdown has begun, so any watching
	// Request.Context().Done() can return early; then wait for them to finish.
	// Handlers that ignore the context still run to completion (graceful) since
	// connections are only closed after the wait.
	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan struct{})
	go func() { s.handlerWG.Wait(); close(done) }()

	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	return err
}

// trackConn adds or removes c from the tracked connection set. On the add
// path it refuses and returns false if the server is shutting down, so a
// connection accepted racing with Close is never leaked; the remove path
// always succeeds.
func (s *Server) trackConn(c net.Conn, add bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		if s.inShutdown.Load() {
			return false
		}
		s.conns[c] = struct{}{}
		return true
	}
	delete(s.conns, c)
	return true
}

// trackListener adds or removes ln from the tracked listener set. On the add
// path it refuses and returns false if the server is shutting down, so a
// listener registered racing with Close is never leaked; the remove path
// always succeeds.
func (s *Server) trackListener(ln *net.Listener, add bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		if s.inShutdown.Load() {
			return false
		}
		s.listeners[ln] = struct{}{}
		return true
	}
	delete(s.listeners, ln)
	return true
}

// ListenAndServe runs a server on addr with a single password and handler. It
// mirrors http.ListenAndServe.
func ListenAndServe(addr, password string, handler Handler) error {
	s := &Server{Addr: addr, Password: password, Handler: handler}
	return s.ListenAndServe()
}
