package rconhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxBodyBytes = 64 << 10

var (
	errNoCommand = errors.New("rconhttp: no command in request")
	errBadBody   = errors.New("rconhttp: invalid request body")
)

// parseCommand extracts the command from a request: a JSON body {"command":...}
// when the content type is JSON, otherwise the raw (trimmed) body, otherwise the
// ?cmd= query parameter.
func parseCommand(r *http.Request) (string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return "", fmt.Errorf("%w: %w", errBadBody, err)
		}
		if body.Command == "" {
			return "", errNoCommand
		}
		return body.Command, nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errBadBody, err)
	}
	if cmd := strings.TrimSpace(string(data)); cmd != "" {
		return cmd, nil
	}
	if cmd := strings.TrimSpace(r.URL.Query().Get("cmd")); cmd != "" {
		return cmd, nil
	}
	return "", errNoCommand
}
