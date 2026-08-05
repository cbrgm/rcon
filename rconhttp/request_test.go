package rconhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		contentType string
		query       string
		want        string
	}{
		{"json", `{"command":"list"}`, "application/json", "", "list"},
		{"text", "say hi", "text/plain", "", "say hi"},
		{"text trims", "  list\n", "text/plain", "", "list"},
		{"query", "", "", "cmd=seed", "seed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/?"+c.query, strings.NewReader(c.body))
			if c.contentType != "" {
				r.Header.Set("Content-Type", c.contentType)
			}
			got, err := parseCommand(r)
			if err != nil {
				t.Fatalf("parseCommand: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseCommandEmpty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	if _, err := parseCommand(r); !errors.Is(err, errNoCommand) {
		t.Fatalf("err = %v, want errNoCommand", err)
	}
}

func TestParseCommandBadJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not json"))
	r.Header.Set("Content-Type", "application/json")
	if _, err := parseCommand(r); !errors.Is(err, errBadBody) {
		t.Fatalf("err = %v, want errBadBody", err)
	}
}
