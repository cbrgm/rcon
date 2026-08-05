package rconclient

import "github.com/cbrgm/rcon/rcon"

// These sentinels are re-exported from github.com/cbrgm/rcon/rcon so callers can
// classify errors without importing the core package. errors.Is matches either
// name.
var (
	// ErrAuthFailed indicates the server rejected the password.
	ErrAuthFailed = rcon.ErrAuthFailed
	// ErrCommandEmpty indicates an empty command was supplied.
	ErrCommandEmpty = rcon.ErrCommandEmpty
	// ErrCommandTooLong indicates the command exceeded the maximum length.
	ErrCommandTooLong = rcon.ErrCommandTooLong
)
