package rconclient_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cbrgm/rcon/rconclient"
)

// For a one-off command, the package-level Execute dials, authenticates, runs
// the command, and closes the connection, all against DefaultClient.
func Example() {
	out, err := rconclient.Execute(context.Background(), "127.0.0.1:25575", "password", "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// Construct a Client to configure timeouts, retries, or logging once, then reuse
// it across goroutines. It mirrors the shape of http.Client.
func ExampleClient() {
	client := rconclient.New(
		rconclient.WithTimeout(10*time.Second),
		rconclient.WithRetry(3, rconclient.ExponentialBackoff(100*time.Millisecond, 2*time.Second)),
	)

	out, err := client.Execute(context.Background(), "127.0.0.1:25575", "password", "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// For many commands against the same server, open a Session instead of dialing
// per command. It keeps one authenticated connection and reconnects on drop.
func ExampleClient_Dial() {
	ctx := context.Background()
	client := rconclient.New()

	session, err := client.Dial(ctx, "127.0.0.1:25575", "password")
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	for _, cmd := range []string{"list", "seed", "save-all"} {
		out, err := session.Execute(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(out)
	}
}

// WithDialer runs the client over any transport you supply, for example a dialer
// with custom timeouts, a SOCKS proxy, or a TLS tunnel, instead of plain TCP.
func ExampleClient_customDialer() {
	dialer := &net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}

	client := rconclient.New(
		rconclient.WithDialer(func(ctx context.Context, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", address)
		}),
	)

	out, err := client.Execute(context.Background(), "127.0.0.1:25575", "password", "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// Classify failures with the re-exported sentinels, so you can tell a rejected
// password from a bad command or a transport problem without importing the core
// package. errors.Is matches through wrapping.
func Example_errorHandling() {
	_, err := rconclient.Execute(context.Background(), "127.0.0.1:25575", "wrong-password", "list")
	switch {
	case err == nil:
		fmt.Println("ok")
	case errors.Is(err, rconclient.ErrAuthFailed):
		fmt.Println("wrong password")
	case errors.Is(err, rconclient.ErrCommandTooLong):
		fmt.Println("command too long")
	default:
		fmt.Println("connection problem:", err)
	}
}
