package rconhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenResolver(t *testing.T) {
	prod := Backend{Addr: "10.0.0.5:25575", Password: "secret"}
	res := TokenResolver(map[string]Backend{"tok_prod": prod})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer tok_prod")
	got, err := res.Resolve(r)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != prod {
		t.Fatalf("got %+v, want %+v", got, prod)
	}
}

func TestTokenResolverUnknown(t *testing.T) {
	res := TokenResolver(map[string]Backend{"tok_prod": {Addr: "x", Password: "y"}})
	for _, h := range []string{"", "Bearer nope", "tok_prod"} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		if _, err := res.Resolve(r); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("Authorization %q: err = %v, want ErrUnauthorized", h, err)
		}
	}
}
