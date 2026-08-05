package rconserver

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// authClient completes the auth handshake over conn and returns once authed.
func authClient(t *testing.T, conn net.Conn, pw string) {
	t.Helper()
	_, _ = (rcon.Packet{ID: 1, Type: rcon.TypeAuth, Body: pw}).WriteTo(conn)
	// read sentinel + auth response
	var p rcon.Packet
	_, _ = p.ReadFrom(conn)
	_, _ = p.ReadFrom(conn)
}

func TestServeConnExecutesCommand(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		if r.Command == "list" {
			_, _ = io.WriteString(w, "3 players")
		}
	})}
	client, server := net.Pipe()
	go s.serveConn(server)

	authClient(t, client, "secret")
	_, _ = (rcon.Packet{ID: 2, Type: rcon.TypeExecCommand, Body: "list"}).WriteTo(client)
	var resp rcon.Packet
	if _, err := resp.ReadFrom(client); err != nil {
		t.Fatal(err)
	}
	if resp.ID != 2 || resp.Body != "3 players" {
		t.Fatalf("resp = %+v", resp)
	}
	_ = client.Close()
}

func TestServeConnEchoesSentinel(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(ResponseWriter, *Request) {})}
	client, server := net.Pipe()
	go s.serveConn(server)

	authClient(t, client, "secret")
	// client's empty RESPONSE_VALUE sentinel
	_, _ = (rcon.Packet{ID: 99, Type: rcon.TypeResponseValue}).WriteTo(client)
	var echo rcon.Packet
	if _, err := echo.ReadFrom(client); err != nil {
		t.Fatal(err)
	}
	if echo.ID != 99 || echo.Type != rcon.TypeResponseValue {
		t.Fatalf("echo = %+v, want empty RESPONSE_VALUE id 99", echo)
	}
	_ = client.Close()
}

func TestServeConnMultiPacketResponse(t *testing.T) {
	body := strings.Repeat("B", 2*rcon.MaxPayloadSize+7)
	s := &Server{Password: "secret", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		_, _ = io.WriteString(w, body)
	})}
	client, server := net.Pipe()
	go s.serveConn(server)

	authClient(t, client, "secret")
	_, _ = (rcon.Packet{ID: 5, Type: rcon.TypeExecCommand, Body: "big"}).WriteTo(client)
	var got strings.Builder
	for got.Len() < len(body) {
		var p rcon.Packet
		if _, err := p.ReadFrom(client); err != nil {
			t.Fatal(err)
		}
		got.WriteString(p.Body)
	}
	if got.String() != body {
		t.Fatal("multi-packet reassembly mismatch")
	}
	_ = client.Close()
}

// startTestServer starts s on a random port and returns its address; it is
// closed via t.Cleanup.
func startTestServer(t *testing.T, s *Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() { _ = s.Close() })
	return ln.Addr().String()
}

func TestServerRoundTripWithClient(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		if r.Command == "list" {
			_, _ = io.WriteString(w, "3/20 players")
		}
	})}
	addr := startTestServer(t, s)

	conn, err := rcon.Dial(t.Context(), addr, "secret")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	out, err := conn.Execute(t.Context(), "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "3/20 players" {
		t.Fatalf("out = %q", out)
	}
}

func TestServerAuthFailureWithClient(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(ResponseWriter, *Request) {})}
	addr := startTestServer(t, s)
	_, err := rcon.Dial(t.Context(), addr, "wrong")
	if !errors.Is(err, rcon.ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

func TestServeValidationError(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	// no handler, no auth
	if err := (&Server{}).Serve(ln); err == nil {
		t.Fatal("Serve should reject an unconfigured server")
	}
}

func TestPackageListenAndServe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the package-level helper via a Server that adopts this listener
	// indirectly: start it, then dial.
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		_, _ = io.WriteString(w, "ok")
	})}
	go func() { _ = s.Serve(ln) }()
	defer s.Close()
	conn, err := rcon.Dial(t.Context(), ln.Addr().String(), "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	out, _ := conn.Execute(t.Context(), "ping")
	if out != "ok" {
		t.Fatalf("out = %q", out)
	}
}

// TestListenAndServeValidatesBeforeListening guards against a file descriptor
// leak: ListenAndServe must reject a misconfigured server via validate()
// before it opens a listening socket. The Addr here is deliberately
// unparseable so that if ListenAndServe called net.Listen first, the error
// returned would be a listen/address error rather than the validate error
// about the nil Handler.
func TestListenAndServeValidatesBeforeListening(t *testing.T) {
	s := &Server{Addr: "not a valid address"}
	err := s.ListenAndServe()
	if err == nil {
		t.Fatal("ListenAndServe should reject an unconfigured server")
	}
	if !strings.Contains(err.Error(), "Handler") {
		t.Fatalf("err = %v, want the validate error about the nil Handler (validate should run before net.Listen)", err)
	}
}

func TestPackageListenAndServeBadAddr(t *testing.T) {
	if err := ListenAndServe("bad:addr:xxx", "pw", HandlerFunc(func(ResponseWriter, *Request) {})); err == nil {
		t.Fatal("expected a listen error for a bad address")
	}
}

// TestServeAfterCloseReturnsImmediately guards against a race where Close
// runs before Serve registers its listener: the listener would then be added
// to the tracked set AFTER Close already closed everything, and Serve would
// block in Accept forever even though Close had already returned. Serve must
// instead see the shutdown flag when it tries to register the listener and
// return ErrServerClosed right away.
func TestServeAfterCloseReturnsImmediately(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(ResponseWriter, *Request) {})}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()

	select {
	case err := <-done:
		if !errors.Is(err, ErrServerClosed) {
			t.Fatalf("Serve returned %v, want ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve blocked in Accept instead of returning ErrServerClosed")
	}
}

// TestServeThenCloseStillReturnsErrServerClosed keeps the ordinary
// serve-then-close path green: a running Serve must still unblock and return
// ErrServerClosed once Close is called on it normally.
func TestServeThenCloseStillReturnsErrServerClosed(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(ResponseWriter, *Request) {})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()

	// Give Serve a moment to register the listener before closing it.
	time.Sleep(10 * time.Millisecond)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, ErrServerClosed) {
			t.Fatalf("Serve returned %v, want ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after Close")
	}
}

func TestShutdownWaitsForInFlightHandler(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})}
	addr := startTestServer(t, s)

	conn, err := rcon.Dial(t.Context(), addr, "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	wg.Go(func() { _, _ = conn.Execute(t.Context(), "slow") })

	<-started // handler is running
	shutErr := make(chan error, 1)
	go func() {
		close(release) // let the handler finish
		shutErr <- s.Shutdown(t.Context())
	}()
	if err := <-shutErr; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	wg.Wait()
}

func TestShutdownTimesOut(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		close(started)
		<-release
	})}
	addr := startTestServer(t, s)
	conn, _ := rcon.Dial(t.Context(), addr, "pw")
	defer conn.Close()
	go func() { _, _ = conn.Execute(t.Context(), "slow") }()
	<-started

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if err := s.Shutdown(ctx); err == nil {
		t.Fatal("Shutdown should time out while a handler is still running")
	}
}

// TestShutdownCancelsRequestContext verifies that Shutdown cancels the request
// context at the start of the drain, so a handler blocking on
// Request.Context().Done() bails out and Shutdown completes gracefully instead
// of deadlocking against a handler that only stops on cancellation.
func TestShutdownCancelsRequestContext(t *testing.T) {
	started := make(chan struct{})
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		close(started)
		<-r.Context().Done()
	})}
	addr := startTestServer(t, s)
	conn, err := rcon.Dial(t.Context(), addr, "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go func() { _, _ = conn.Execute(t.Context(), "wait") }()
	<-started

	done := make(chan error, 1)
	go func() { done <- s.Shutdown(t.Context()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown = %v, want nil (handler bailed via context)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown blocked: request context was not cancelled at shutdown start")
	}
}

func TestRequestContextCancelledOnClose(t *testing.T) {
	cancelled := make(chan struct{})
	started := make(chan struct{})
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	})}
	addr := startTestServer(t, s)
	conn, _ := rcon.Dial(t.Context(), addr, "pw")
	defer conn.Close()
	go func() { _, _ = conn.Execute(t.Context(), "wait") }()
	<-started
	_ = s.Close()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("request context was not cancelled on Close")
	}
}

func TestServerConcurrentClients(t *testing.T) {
	s := &Server{Password: "pw", Handler: HandlerFunc(func(w ResponseWriter, r *Request) {
		_, _ = io.WriteString(w, "pong")
	})}
	addr := startTestServer(t, s)

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			conn, err := rcon.Dial(t.Context(), addr, "pw")
			if err != nil {
				t.Errorf("Dial: %v", err)
				return
			}
			defer conn.Close()
			out, err := conn.Execute(t.Context(), "ping")
			if err != nil || out != "pong" {
				t.Errorf("Execute = %q, %v", out, err)
			}
		})
	}
	wg.Wait()
}

func TestServeConnRecoversHandlerPanic(t *testing.T) {
	s := &Server{Password: "secret", Handler: HandlerFunc(func(ResponseWriter, *Request) {
		panic("boom")
	})}
	client, server := net.Pipe()
	go s.serveConn(server)

	authClient(t, client, "secret")
	_, _ = (rcon.Packet{ID: 2, Type: rcon.TypeExecCommand, Body: "x"}).WriteTo(client)
	// connection should be closed after the panic; a read returns an error.
	var p rcon.Packet
	if _, err := p.ReadFrom(client); err == nil {
		t.Fatal("expected the connection to close after a handler panic")
	}
	_ = client.Close()
}

// TestReadTimeoutBoundsAuthHandshake guards against a slowloris-style
// unauthenticated client: a connection that never sends its auth packet must
// still be closed once ReadTimeout elapses, rather than pinning a goroutine
// and file descriptor forever.
func TestReadTimeoutBoundsAuthHandshake(t *testing.T) {
	s := &Server{
		Password:    "secret",
		Handler:     HandlerFunc(func(ResponseWriter, *Request) {}),
		ReadTimeout: 50 * time.Millisecond,
	}
	addr := startTestServer(t, s)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send nothing. The server should close the connection once ReadTimeout
	// elapses instead of blocking forever on the auth read.
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := conn.Read(buf)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the connection to be closed after ReadTimeout elapsed")
		}
	case <-time.After(time.Second):
		t.Fatal("server did not close the unauthenticated connection within ReadTimeout")
	}
}
