package rconhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
)

func TestServeHTTPSuccessJSON(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	h := New(Backend{Addr: srv.Addr(), Password: "secret"})
	defer h.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"command":"list"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body)
	}
	var got result
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.Response != "3 players" {
		t.Fatalf("got %+v", got)
	}
}

func TestServeHTTPSuccessText(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	h := New(Backend{Addr: srv.Addr(), Password: "secret"})
	defer h.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list"))
	req.Header.Set("Accept", "text/plain")
	h.ServeHTTP(w, req)

	if body, _ := io.ReadAll(w.Body); strings.TrimSpace(string(body)) != "3 players" {
		t.Fatalf("body = %q", body)
	}
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	h := New(Backend{Addr: "x:1", Password: "y"})
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d", w.Code)
	}
	if a := w.Header().Get("Allow"); a != "POST" {
		t.Fatalf("Allow = %q", a)
	}
}

func TestServeHTTPEmptyCommand(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	h := New(Backend{Addr: srv.Addr(), Password: "secret"})
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("")))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServeHTTPTokenResolverUnauthorized(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	h := New(Backend{}, WithResolver(TokenResolver(map[string]Backend{
		"tok": {Addr: srv.Addr(), Password: "secret"},
	})))
	defer h.Close()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list"))
	req.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServeHTTPResolverSelectsBackend(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("who", "picked")
	h := New(Backend{}, WithResolver(TokenResolver(map[string]Backend{
		"tok": {Addr: srv.Addr(), Password: "secret"},
	})))
	defer h.Close()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("who"))
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body)
	}
}

func TestServeHTTPBackendDown(t *testing.T) {
	h := New(Backend{Addr: "127.0.0.1:1", Password: "x"})
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list")))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServeHTTPWrongPassword(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	h := New(Backend{Addr: srv.Addr(), Password: "wrong"})
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list")))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServeHTTPNoBackendConfigured(t *testing.T) {
	h := New(Backend{}) // empty backend, no resolver
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list")))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServeHTTPLogsDialFailure(t *testing.T) {
	const password = "supersecret"
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := New(Backend{Addr: "127.0.0.1:1", Password: password}, WithLogger(l))
	defer h.Close()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list")))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", w.Code)
	}

	logged := buf.String()
	if !strings.Contains(logged, "rcon backend dial failed") {
		t.Fatalf("missing dial-failure log: %q", logged)
	}
	if !strings.Contains(logged, "127.0.0.1:1") {
		t.Fatalf("missing addr in log: %q", logged)
	}
	if strings.Contains(logged, password) {
		t.Fatalf("password leaked into log: %q", logged)
	}
}

func TestServeHTTPOversizedBody(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "ok")
	h := New(Backend{Addr: srv.Addr(), Password: "secret"})
	defer h.Close()

	w := httptest.NewRecorder()
	body := strings.Repeat("a", maxBodyBytes+1)
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
}

func TestServeHTTPReconnectAfterDrop(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "ok")
	h := New(Backend{Addr: srv.Addr(), Password: "secret"})
	defer h.Close()

	do := func() int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list")))
		return w.Code
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("first code = %d", code)
	}
	srv.DropNext() // server drops the next connection once
	// The session may fail once and reconnect; a follow-up request must succeed.
	_ = do()
	if code := do(); code != http.StatusOK {
		t.Fatalf("after drop, code = %d", code)
	}
}
