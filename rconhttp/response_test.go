package rconhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbrgm/rcon/rcon"
)

func TestWriteResultJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	writeResult(w, r, "list", "3 players")

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var got result
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Command != "list" || got.Response != "3 players" {
		t.Fatalf("got %+v", got)
	}
}

func TestWriteResultText(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Accept", "text/plain")
	writeResult(w, r, "list", "3 players")

	if body, _ := io.ReadAll(w.Body); string(body) != "3 players" {
		t.Fatalf("body = %q", body)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	writeError(w, r, http.StatusBadGateway, errors.New("boom"))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", w.Code)
	}
	var got errResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "boom" {
		t.Fatalf("error = %q", got.Error)
	}
}

func TestExecuteStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{rcon.ErrCommandTooLong, http.StatusBadRequest},
		{rcon.ErrCommandEmpty, http.StatusBadRequest},
		{context.DeadlineExceeded, http.StatusGatewayTimeout},
		{rcon.ErrAuthFailed, http.StatusBadGateway},
		{&net.OpError{Op: "dial", Err: errors.New("refused")}, http.StatusServiceUnavailable},
		{timeoutErr{}, http.StatusGatewayTimeout},
		{errors.New("weird"), http.StatusBadGateway},
	}
	for _, c := range cases {
		if got := executeStatus(c.err); got != c.want {
			t.Errorf("executeStatus(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

// timeoutErr is a net.Error whose Timeout reports true, standing in for a raw
// socket i/o timeout (distinct from context.DeadlineExceeded).
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }
