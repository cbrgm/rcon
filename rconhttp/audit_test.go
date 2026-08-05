package rconhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeHTTPResolverErrorMapsTo400(t *testing.T) {
	boom := errors.New("resolver boom")
	h := New(Backend{}, WithResolver(ResolverFunc(func(*http.Request) (Backend, error) {
		return Backend{}, boom
	})))
	defer h.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("list"))
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 for a non-unauthorized resolver error", w.Code)
	}
}

func TestHandlerCloseIdempotent(t *testing.T) {
	h := New(Backend{Addr: "x:1", Password: "y"})
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
