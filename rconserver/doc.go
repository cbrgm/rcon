// Package rconserver builds Source RCON servers the way net/http builds HTTP
// servers: write a Handler, hand it to a Server, call ListenAndServe.
//
// It speaks the RCON wire protocol (authentication, command framing, multi-packet
// chunking) so a handler only turns a command into a response. It is built on
// github.com/cbrgm/rcon and, like the rest of the module, depends only on the
// standard library.
//
// A Server must be given a Handler and either a Password or an Authenticator;
// it refuses to run otherwise, to avoid an accidentally open server. A handler
// may be invoked concurrently for different connections, so shared state must be
// safe for concurrent use.
package rconserver
