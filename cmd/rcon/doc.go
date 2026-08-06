// Command rcon is a small command-line client for the Source RCON protocol.
//
// It runs a single command and exits, or drops into an interactive prompt when
// no command is given. Connection settings come from flags, then environment
// variables (RCON_HOST, RCON_PORT, RCON_PASSWORD, RCON_SINGLE_PACKET,
// RCON_DRAIN), then an optional JSON config of named servers, in that order of
// precedence.
//
// Usage:
//
//	rcon [flags] [command...]
//
// Examples:
//
//	rcon --host 127.0.0.1 --port 25575 --password secret list
//	rcon --server prod
//	rcon --drain --host 127.0.0.1 --port 27015 --password changeme players
//
// Run "rcon --help" for the full flag list.
package main
