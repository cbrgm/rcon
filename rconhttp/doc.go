// Package rconhttp serves RCON over HTTP with the standard net/http server.
//
// It turns HTTP requests into RCON commands against a backend, so a web
// frontend, a script, or curl can administer a game server without speaking the
// RCON wire protocol. It is built on github.com/cbrgm/rcon/rconclient and, like
// the rest of the module, depends only on the standard library.
//
// The zero value of Handler is not usable; construct one with New.
//
// The handler executes administrative commands: always place it behind your own
// authentication middleware and serve it over TLS. Never expose it
// unauthenticated.
package rconhttp
