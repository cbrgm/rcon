package rconhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/cbrgm/rcon/rcon"
)

var (
	errMethodNotAllowed = errors.New("rconhttp: method not allowed")
	errNoBackend        = errors.New("rconhttp: resolved backend has no address")
)

type result struct {
	Command  string `json:"command"`
	Response string `json:"response"`
}

type errResult struct {
	Error string `json:"error"`
}

// wantsText reports whether the client asked for a plain-text response.
func wantsText(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/plain")
}

// writeResult renders a successful command result as JSON or plain text.
func writeResult(w http.ResponseWriter, r *http.Request, command, response string) {
	if wantsText(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, response)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result{Command: command, Response: response})
}

// writeError renders err at the given status as JSON or plain text.
func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	if wantsText(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "error: %s\n", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errResult{Error: err.Error()})
}

// executeStatus maps a command-execution error to an HTTP status.
func executeStatus(err error) int {
	switch {
	case errors.Is(err, rcon.ErrCommandEmpty), errors.Is(err, rcon.ErrCommandTooLong):
		return http.StatusBadRequest
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case errors.Is(err, rcon.ErrAuthFailed):
		return http.StatusBadGateway
	}
	if netErr, ok := errors.AsType[net.Error](err); ok {
		if netErr.Timeout() {
			return http.StatusGatewayTimeout
		}
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}
