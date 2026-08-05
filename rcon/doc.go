// Package rcon implements the Source RCON protocol for administering dedicated
// game servers over TCP.
//
// It is the low-level foundation of this module: a single authenticated
// connection ([Conn]) with explicit, one-command-at-a-time semantics. Higher
// level ergonomics (a default client, retries, sessions) live in the
// github.com/cbrgm/rcon/rconclient package, which is built on top of this one.
//
// The zero value of [Conn] is not usable; obtain one with [Dial] or [Open].
package rcon
