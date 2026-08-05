package rconhttp_test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/cbrgm/rcon/rconhttp"
	"github.com/cbrgm/rcon/rconserver"
)

// Serve a single fixed backend. Wrap the handler with your own auth middleware
// and serve it over TLS; it runs administrative commands.
func Example() {
	h := rconhttp.New(rconhttp.Backend{
		Addr:     "127.0.0.1:25575",
		Password: os.Getenv("RCON_PASSWORD"),
	})
	defer h.Close()

	mux := http.NewServeMux()
	mux.Handle("POST /command", h)
	_ = http.ListenAndServe(":8080", mux)
}

// Switch backends per request with a bearer token that maps to a server-side
// backend, so the RCON password never crosses the wire.
func ExampleTokenResolver() {
	h := rconhttp.New(rconhttp.Backend{}, rconhttp.WithResolver(
		rconhttp.TokenResolver(map[string]rconhttp.Backend{
			"tok_prod": {Addr: "10.0.0.5:25575", Password: os.Getenv("PROD_PW")},
			"tok_stg":  {Addr: "10.0.0.6:25575", Password: os.Getenv("STG_PW")},
		}),
	))
	defer h.Close()

	mux := http.NewServeMux()
	mux.Handle("POST /command", h)
	_ = http.ListenAndServe(":8080", mux)
}

// A ResolverFunc picks the backend however you like, here from a path segment,
// with the password kept server-side. Return ErrUnauthorized to map to HTTP 401.
func ExampleResolverFunc() {
	servers := map[string]rconhttp.Backend{
		"prod": {Addr: "10.0.0.5:25575", Password: os.Getenv("PROD_PW")},
		"stg":  {Addr: "10.0.0.6:25575", Password: os.Getenv("STG_PW")},
	}

	resolve := rconhttp.ResolverFunc(func(r *http.Request) (rconhttp.Backend, error) {
		b, ok := servers[r.PathValue("server")]
		if !ok {
			return rconhttp.Backend{}, rconhttp.ErrUnauthorized
		}
		return b, nil
	})

	h := rconhttp.New(rconhttp.Backend{}, rconhttp.WithResolver(resolve))
	defer h.Close()

	mux := http.NewServeMux()
	mux.Handle("POST /servers/{server}/command", h)
	_ = http.ListenAndServe(":8080", mux)
}

// A complete round trip: an RCON server, exposed over HTTP by the handler, and a
// client that POSTs a command and reads the JSON result.
func Example_httpRoundTrip() {
	// A backend RCON server on an ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	backend := &rconserver.Server{
		Password: "secret",
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			_, _ = io.WriteString(w, "3/20 players online")
		}),
	}
	go func() { _ = backend.Serve(ln) }()
	defer backend.Close()

	// Expose it over HTTP, fronted by a test server.
	h := rconhttp.New(rconhttp.Backend{Addr: ln.Addr().String(), Password: "secret"})
	defer h.Close()
	front := httptest.NewServer(h)
	defer front.Close()

	// The command is the request body; the reply comes back as JSON.
	resp, err := http.Post(front.URL, "text/plain", strings.NewReader("list"))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Print(string(body))
	// Output: {"command":"list","response":"3/20 players online"}
}
