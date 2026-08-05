package rconhttp

import (
	"errors"
	"net/http"
	"strings"
)

// Backend is an RCON server a Handler talks to. The password is held
// server-side or produced by a Resolver; it never has to cross the HTTP
// boundary.
type Backend struct {
	// Addr is the RCON server address as "host:port".
	Addr string
	// Password is the RCON password.
	Password string
}

// Resolver selects the Backend for a request. Returning an error rejects the
// request; return ErrUnauthorized to map it to HTTP 401.
type Resolver interface {
	Resolve(r *http.Request) (Backend, error)
}

// ResolverFunc adapts an ordinary function to a Resolver.
type ResolverFunc func(r *http.Request) (Backend, error)

// Resolve calls f(r).
func (f ResolverFunc) Resolve(r *http.Request) (Backend, error) { return f(r) }

// ErrUnauthorized, when returned by a Resolver, maps to HTTP 401.
var ErrUnauthorized = errors.New("rconhttp: unauthorized")

// TokenResolver resolves an "Authorization: Bearer <token>" header to a Backend
// via byToken, so the RCON password never crosses the wire. A missing or
// unknown token yields ErrUnauthorized. It is the recommended way to switch
// backends dynamically.
func TokenResolver(byToken map[string]Backend) Resolver {
	return ResolverFunc(func(r *http.Request) (Backend, error) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			return Backend{}, ErrUnauthorized
		}
		b, ok := byToken[strings.TrimPrefix(auth, prefix)]
		if !ok {
			return Backend{}, ErrUnauthorized
		}
		return b, nil
	})
}

// fixedResolver always returns the same Backend.
type fixedResolver struct{ b Backend }

// Resolve returns f's backend, ignoring r.
func (f fixedResolver) Resolve(*http.Request) (Backend, error) { return f.b, nil }
