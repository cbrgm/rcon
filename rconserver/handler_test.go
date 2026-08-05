package rconserver

import (
	"context"
	"testing"
)

func TestRequestContextDefaultsToBackground(t *testing.T) {
	r := &Request{Command: "list"}
	if r.Context() != context.Background() {
		t.Fatal("Context() should default to context.Background() when unset")
	}
}

func TestHandlerFuncServeRCON(t *testing.T) {
	called := false
	var h Handler = HandlerFunc(func(w ResponseWriter, r *Request) { called = true })
	h.ServeRCON(nil, &Request{Command: "x"})
	if !called {
		t.Fatal("HandlerFunc did not invoke the wrapped function")
	}
}

func TestServerLoggerNeverNil(t *testing.T) {
	s := &Server{}
	if s.logger() == nil {
		t.Fatal("logger() must never return nil")
	}
}
