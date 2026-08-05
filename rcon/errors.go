package rcon

import "errors"

// Sentinel errors returned by this package. Wrap-aware: test with errors.Is.
var (
	// ErrResponseTooShort means a frame declared a Size below MinPacketSize.
	ErrResponseTooShort = errors.New("rcon: response smaller than minimum packet")
	// ErrResponseTooLong means a frame declared a Size above the protocol max.
	ErrResponseTooLong = errors.New("rcon: response larger than maximum packet")
)

var (
	// ErrAuthFailed means the server rejected the password (auth response ID -1).
	ErrAuthFailed = errors.New("rcon: authentication failed")
	// ErrInvalidAuthResponse means the auth reply had an unexpected type.
	ErrInvalidAuthResponse = errors.New("rcon: unexpected auth response type")
)

// ErrClosed is returned by operations on a closed Conn.
var ErrClosed = errors.New("rcon: connection closed")

var (
	// ErrCommandEmpty means Execute was called with an empty command.
	ErrCommandEmpty = errors.New("rcon: command is empty")
	// ErrCommandTooLong means the command exceeded the configured max length.
	ErrCommandTooLong = errors.New("rcon: command too long")
	// ErrResponseMismatch means a response packet's ID did not match the request.
	ErrResponseMismatch = errors.New("rcon: response id did not match request")
)
