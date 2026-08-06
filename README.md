# 🕹️ rcon

**Pure modern Go implementation of the [RCON protocol](https://developer.valvesoftware.com/wiki/Source_RCON_Protocol) for administering game servers over TCP.**

[![GitHub release](https://img.shields.io/github/release/cbrgm/rcon.svg)](https://github.com/cbrgm/rcon)
[![Go Reference](https://pkg.go.dev/badge/github.com/cbrgm/rcon.svg)](https://pkg.go.dev/github.com/cbrgm/rcon)
[![go-lint-test](https://github.com/cbrgm/rcon/actions/workflows/go-lint-test.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/go-lint-test.yml)
[![go-binaries](https://github.com/cbrgm/rcon/actions/workflows/go-binaries.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/go-binaries.yml)
[![container](https://github.com/cbrgm/rcon/actions/workflows/container.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/container.yml)

RCON lets you send admin commands to a running game server (Minecraft, Source engine games, Rust, and others). This module gives you the protocol as a reusable library and a small CLI built on top of it.

It comes in five parts, layered so each one builds on the one before:

| Package | What it does | Coverage |
| --- | --- | --- |
| `rcon` | The low-level protocol: one authenticated connection, one command at a time. | ![coverage](https://img.shields.io/badge/coverage-87.8%25-brightgreen) |
| `rconclient` | A higher-level client in the shape of `net/http`: a `DefaultClient`, package-level helpers, retries, and sessions for repeated commands. | ![coverage](https://img.shields.io/badge/coverage-89.4%25-brightgreen) |
| `cmd/rcon` | The CLI, for single-shot and interactive use. | ![coverage](https://img.shields.io/badge/coverage-92.2%25-brightgreen) |
| `rconhttp` | An `http.Handler` that turns HTTP requests into RCON commands, so you can serve RCON to a frontend or a script without it speaking the wire protocol. | ![coverage](https://img.shields.io/badge/coverage-91.2%25-brightgreen) |
| `rconserver` | The other direction: build an RCON server the way `net/http` builds an HTTP one. Write a `Handler`, hand it to a `Server`, call `ListenAndServe`. | ![coverage](https://img.shields.io/badge/coverage-93.0%25-brightgreen) |

## Documentation

The full API, with runnable examples for every entry point, lives on pkg.go.dev. The snippets below are complete programs, copy one and run it.

- Core protocol: **[pkg.go.dev/github.com/cbrgm/rcon/rcon](https://pkg.go.dev/github.com/cbrgm/rcon/rcon)**
- Client: **[pkg.go.dev/github.com/cbrgm/rcon/rconclient](https://pkg.go.dev/github.com/cbrgm/rcon/rconclient)**
- Serve RCON over HTTP: **[pkg.go.dev/github.com/cbrgm/rcon/rconhttp](https://pkg.go.dev/github.com/cbrgm/rcon/rconhttp)**
- Build an RCON server: **[pkg.go.dev/github.com/cbrgm/rcon/rconserver](https://pkg.go.dev/github.com/cbrgm/rcon/rconserver)**

## Install

As a library:

```
go get github.com/cbrgm/rcon
```

As a CLI:

```
go install github.com/cbrgm/rcon/cmd/rcon@latest
```

## Library

A one-off command through the default client. It dials, authenticates, runs the command, and closes the connection:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cbrgm/rcon/rconclient"
)

func main() {
	out, err := rconclient.Execute(context.Background(), "127.0.0.1:25575", "password", "list")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
```

For many commands against one server, build a `Client` once (timeouts, retries, logging) and open a `Session` that keeps a single connection and reconnects on drop:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cbrgm/rcon/rconclient"
)

func main() {
	ctx := context.Background()

	client := rconclient.New(
		rconclient.WithTimeout(10*time.Second),
		rconclient.WithRetry(3, rconclient.ExponentialBackoff(100*time.Millisecond, 2*time.Second)),
	)

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
		fmt.Printf("%s -> %s\n", cmd, out)
	}
}
```

Or drop down to the core `rcon` package for a single connection you manage yourself:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cbrgm/rcon/rcon"
)

func main() {
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
```

## Game servers and packet modes

By default the client reassembles multi-packet responses using the Source terminator sentinel. That is correct for Source-engine servers (CS2, TF2, Garry's Mod) and Minecraft, where a large reply can span several packets. Some game servers mishandle that sentinel, so there are two escape hatches:

- `WithSinglePacket()` / `--single-packet`: read exactly one reply packet per command. For servers that mishandle the terminator and never split a reply.
- `WithReadUntilIdle(window)` / `--drain`: read reply packets until the connection goes quiet. For servers like **Project Zomboid** that split large replies (e.g. `help`) but still mishandle the terminator.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cbrgm/rcon/rconclient"
)

func main() {
	// Project Zomboid splits large replies but mishandles the terminator, so
	// read until the connection goes idle instead of waiting for a terminator.
	client := rconclient.New(rconclient.WithReadUntilIdle(0)) // 0 => default 100ms window

	out, err := client.Execute(context.Background(), "127.0.0.1:27015", "changeme", "players")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
```

The same from the CLI:

```
rcon --drain --host 127.0.0.1 --port 27015 --password changeme         # Project Zomboid
rcon --single-packet --host 127.0.0.1 --port 27015 --password secret
```

## CLI

Run one command and exit, or leave the command off to drop into an interactive prompt:

```
rcon --host 127.0.0.1 --port 25575 --password secret list
rcon --server prod          # interactive REPL against a named server
```

Config comes from flags, then environment variables (`RCON_HOST`, `RCON_PORT`, `RCON_PASSWORD`, `RCON_SINGLE_PACKET`, `RCON_DRAIN`), then an optional JSON file, in that order of precedence. The file holds named servers so you don't have to retype connection details:

```json
{
	"default": "prod",
	"servers": {
		"prod": { "host": "rcon.example.com", "port": 25575, "password": "secret" },
		"zomboid": { "host": "127.0.0.1", "port": 27015, "password": "changeme", "drain": true }
	}
}
```

Run `rcon --help` for the full flag list.

## Serve over HTTP

Expose RCON over HTTP by mounting `rconhttp.New` on any `http.ServeMux`:

```go
package main

import (
	"log"
	"net/http"

	"github.com/cbrgm/rcon/rconhttp"
)

func main() {
	h := rconhttp.New(rconhttp.Backend{
		Addr:     "127.0.0.1:25575",
		Password: "secret",
	})
	defer h.Close()

	mux := http.NewServeMux()
	mux.Handle("POST /command", h)

	// Put this behind your own auth and TLS; it runs administrative commands.
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Then call it with the command in the request body:

```
curl -sS -XPOST --data 'list' http://localhost:8080/command
# {"command":"list","response":"..."}
```

Never expose it as is. For dynamic backends resolved per request (e.g. a bearer token that maps to a server, so the password stays server-side), see the `TokenResolver` example on the [rconhttp docs](https://pkg.go.dev/github.com/cbrgm/rcon/rconhttp).

## Build a server

`rconserver` builds an RCON server the way `net/http` builds an HTTP one:

```go
package main

import (
	"io"
	"log"

	"github.com/cbrgm/rcon/rconserver"
)

func main() {
	srv := &rconserver.Server{
		Addr:     ":25575",
		Password: "secret",
		Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
			switch r.Command {
			case "list":
				io.WriteString(w, "3/20 players online")
			default:
				io.WriteString(w, "unknown command: "+r.Command)
			}
		}),
	}
	log.Fatal(srv.ListenAndServe())
}
```

A `Server` needs a `Handler` and either a `Password` or an `Authenticator`, otherwise it refuses to run.

## Contributing & License

- Contributions are welcome. Open an issue or a PR.
- Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
