package rcon_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cbrgm/rcon/rcon"
)

// Connect to a server, run a command, and print the response.
func Example() {
	ctx := context.Background()

	conn, err := rcon.Dial(ctx, "127.0.0.1:25575", "password")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	out, err := conn.Execute(ctx, "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// Dial accepts functional options, for example to tighten the dial and
// per-command timeouts.
func ExampleDial_timeout() {
	ctx := context.Background()

	conn, err := rcon.Dial(ctx, "127.0.0.1:25575", "password",
		rcon.WithDialTimeout(2*time.Second),
		rcon.WithDeadline(3*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	out, err := conn.Execute(ctx, "status")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// Open wraps a connection you established yourself, so you can reach the server
// through a proxy, a TLS tunnel, or a custom dialer instead of Dial's plain TCP.
func ExampleOpen() {
	ctx := context.Background()

	netConn, err := net.Dial("tcp", "127.0.0.1:25575")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := rcon.Open(ctx, netConn, "password")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	out, err := conn.Execute(ctx, "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}

// A single Conn runs many commands in sequence. Concurrent calls are safe but
// serialized, since RCON has no request multiplexing.
func ExampleConn_Execute() {
	ctx := context.Background()

	conn, err := rcon.Dial(ctx, "127.0.0.1:25575", "password")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for _, cmd := range []string{"seed", "time query day", "list"} {
		out, err := conn.Execute(ctx, cmd)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(out)
	}
}

// Packet is the wire frame. You only need it to speak the protocol yourself;
// Conn handles framing for you. Here a command request is encoded to bytes and
// decoded straight back.
func ExamplePacket() {
	var buf bytes.Buffer

	req := rcon.Packet{ID: 42, Type: rcon.TypeExecCommand, Body: "list"}
	if _, err := req.WriteTo(&buf); err != nil {
		log.Fatal(err)
	}

	var got rcon.Packet
	if _, err := got.ReadFrom(&buf); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("id=%d type=%d body=%q\n", got.ID, got.Type, got.Body)
	// Output: id=42 type=2 body="list"
}
