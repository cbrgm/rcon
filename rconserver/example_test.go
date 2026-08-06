package rconserver_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconserver"
)

// Serve RCON with a single password and a handler.
func Example() {
	srv := &rconserver.Server{
		Addr:     ":25575",
		Password: os.Getenv("RCON_PASSWORD"),
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			switch firstWord(r.Command) {
			case "list":
				_, _ = io.WriteString(w, "There are 3/20 players online")
			default:
				_, _ = io.WriteString(w, "unknown command")
			}
		}),
	}
	log.Fatal(srv.ListenAndServe())
}

// ListenAndServe is the one-liner form.
func ExampleListenAndServe() {
	h := rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
		_, _ = io.WriteString(w, "pong")
	})
	log.Fatal(rconserver.ListenAndServe(":25575", os.Getenv("RCON_PASSWORD"), h))
}

// An Authenticator validates the password yourself, for example to accept
// several passwords or look one up dynamically, instead of one shared Password.
func ExampleServer_authenticator() {
	allowed := map[string]bool{
		os.Getenv("ADMIN_PW"):  true,
		os.Getenv("DEPLOY_PW"): true,
	}
	delete(allowed, "") // never accept an unset password

	srv := &rconserver.Server{
		Addr: ":25575",
		Authenticator: func(password string) bool {
			return allowed[password]
		},
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			_, _ = io.WriteString(w, "ok")
		}),
	}
	log.Fatal(srv.ListenAndServe())
}

// Shutdown stops accepting connections and drains in-flight handlers, bounded by
// a context. Here it is triggered by SIGINT.
func ExampleServer_Shutdown() {
	srv := &rconserver.Server{
		Addr:     ":25575",
		Password: os.Getenv("RCON_PASSWORD"),
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			_, _ = io.WriteString(w, "pong")
		}),
	}

	go func() {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		<-ctx.Done()

		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != rconserver.ErrServerClosed {
		log.Fatal(err)
	}
}

// A complete round trip: start a server on an ephemeral port and run a command
// against it with a client.
func Example_roundTrip() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	srv := &rconserver.Server{
		Password: "secret",
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			if r.Command == "ping" {
				_, _ = io.WriteString(w, "pong")
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	ctx := context.Background()
	conn, err := rcon.Dial(ctx, ln.Addr().String(), "secret")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	out, err := conn.Execute(ctx, "ping")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
	// Output: pong
}

// A HandlerFunc dispatches on the command and can read the client's address and
// the per-request context, which is canceled when the server begins shutting
// down.
func ExampleHandlerFunc() {
	h := rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
		log.Printf("command %q from %s", r.Command, r.RemoteAddr)

		select {
		case <-r.Context().Done():
			return // server is shutting down; abandon the response
		default:
		}
		_, _ = io.WriteString(w, "ok")
	})
	log.Fatal(rconserver.ListenAndServe(":25575", os.Getenv("RCON_PASSWORD"), h))
}

func firstWord(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
