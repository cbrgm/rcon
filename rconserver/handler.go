package rconserver

import (
	"context"
	"io"
)

// Handler responds to a single authenticated RCON command.
type Handler interface {
	ServeRCON(w ResponseWriter, r *Request)
}

// HandlerFunc adapts an ordinary function to a Handler.
type HandlerFunc func(w ResponseWriter, r *Request)

// ServeRCON calls f(w, r).
func (f HandlerFunc) ServeRCON(w ResponseWriter, r *Request) { f(w, r) }

// Request is one command from an authenticated client.
type Request struct {
	// Command is the command body sent by the client.
	Command string
	// RemoteAddr is the client's network address.
	RemoteAddr string

	ctx context.Context
}

// Context returns the request's context. It is canceled when the server begins
// shutting down. It never returns nil; an unset context reports
// context.Background.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// ResponseWriter accumulates a command's response. The server frames the written
// bytes into one or more RESPONSE_VALUE packets, splitting bodies larger than the
// protocol's payload cap. RCON has no headers or status, so this is just a writer.
type ResponseWriter interface {
	io.Writer
	WriteString(s string) (int, error)
}
