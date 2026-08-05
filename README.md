# 🎮 rcon

**A pure Go implementation of the [Source RCON protocol](https://developer.valvesoftware.com/wiki/Source_RCON_Protocol) for administering game servers over TCP, plus a small CLI on top. Zero third-party dependencies, stdlib only.**

[![GitHub release](https://img.shields.io/github/release/cbrgm/rcon.svg)](https://github.com/cbrgm/rcon)
[![Go Reference](https://pkg.go.dev/badge/github.com/cbrgm/rcon.svg)](https://pkg.go.dev/github.com/cbrgm/rcon)
[![go-lint-test](https://github.com/cbrgm/rcon/actions/workflows/go-lint-test.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/go-lint-test.yml)
[![go-binaries](https://github.com/cbrgm/rcon/actions/workflows/go-binaries.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/go-binaries.yml)
[![container](https://github.com/cbrgm/rcon/actions/workflows/container.yml/badge.svg)](https://github.com/cbrgm/rcon/actions/workflows/container.yml)

RCON lets you send admin commands to a running game server (Minecraft, Source engine games, Rust, and others). This module gives you the protocol as a reusable library and a small CLI built on top of it.

It comes in five parts, layered so each one builds on the one before:

- `rcon` -> the low-level protocol: one authenticated connection, one command at a time.
- `rconclient` -> a higher-level client in the shape of `net/http`: a `DefaultClient`, package-level helpers, retries, and sessions for repeated commands.
- `cmd/rcon` -> the CLI, for single-shot and interactive use.
- `rconhttp` -> an `http.Handler` that turns HTTP requests into RCON commands, so you can serve RCON to a frontend or a script without it speaking the wire protocol.
- `rconserver` -> the other direction: build an RCON server the way `net/http` builds an HTTP one, write a `Handler`, hand it to a `Server`, call `ListenAndServe`.

Every package carries its own tests. Statement coverage per package:

![rcon coverage](https://img.shields.io/badge/rcon-87.8%25-brightgreen)
![rconclient coverage](https://img.shields.io/badge/rconclient-89.4%25-brightgreen)
![rconhttp coverage](https://img.shields.io/badge/rconhttp-91.2%25-brightgreen)
![rconserver coverage](https://img.shields.io/badge/rconserver-93.0%25-brightgreen)
![cmd/rcon coverage](https://img.shields.io/badge/cmd%2Frcon-92.2%25-brightgreen)

## Documentation

The full API, with runnable examples for every entry point, lives on pkg.go.dev. The snippets below just get you started.

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

A one-off command through the default client:

```go
out, err := rconclient.Execute(ctx, "127.0.0.1:25575", "password", "list")
```

Or drop down to the core package for a connection you manage yourself:

```go
conn, err := rcon.Dial(ctx, "127.0.0.1:25575", "password")
if err != nil {
	log.Fatal(err)
}
defer conn.Close()

out, err := conn.Execute(ctx, "list")
```

For retries, timeouts, logging, and persistent sessions, see the `rconclient` docs linked above.

## CLI

Run one command and exit, or leave the command off to drop into an interactive prompt:

```
rcon --host 127.0.0.1 --port 25575 --password secret list
rcon --server prod          # interactive REPL against a named server
```

Config comes from flags, then environment variables (`RCON_HOST`, `RCON_PORT`, `RCON_PASSWORD`), then an optional JSON file, in that order of precedence. The file holds named servers so you don't have to retype connection details:

```json
{
	"default": "prod",
	"servers": {
		"prod": { "host": "rcon.example.com", "port": 25575, "password": "secret" }
	}
}
```

Run `rcon --help` for the full flag list.

## Serve over HTTP

Mount `rconhttp.New` on any `http.ServeMux` to expose RCON over HTTP:

```go
mux := http.NewServeMux()
mux.Handle("POST /command", rconhttp.New(rconhttp.Backend{
	Addr:     "127.0.0.1:25575",
	Password: "secret",
}))
http.ListenAndServe(":8080", mux)
```

It runs administrative commands, so put it behind your own auth and TLS. Never expose it as is. For dynamic backends resolved per request (e.g. a bearer token that maps to a server, so the password stays server-side), see the `TokenResolver` example on the [rconhttp docs](https://pkg.go.dev/github.com/cbrgm/rcon/rconhttp).

## Build a server

`rconserver` builds an RCON server the way `net/http` builds an HTTP one:

```go
srv := &rconserver.Server{
	Addr:     ":25575",
	Password: "secret",
	Handler: rconserver.HandlerFunc(func(w rconserver.ResponseWriter, r *rconserver.Request) {
		io.WriteString(w, "3/20 players online")
	}),
}
log.Fatal(srv.ListenAndServe())
```

A `Server` needs a `Handler` and either a `Password` or an `Authenticator`, otherwise it refuses to run.

## Contributing & License

- Contributions are welcome. Open an issue or a PR.
- Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
