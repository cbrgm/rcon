package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cbrgm/rcon/rconclient"
)

// Build metadata, injected via -ldflags by the Makefile.
var (
	Version   = "dev"
	Revision  = "none"
	BuildDate = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses args and drives the CLI over the given streams, returning a process
// exit code. It is separated from main and uses its own flag set so it can be
// tested with injected arguments and IO.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rcon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		host       = fs.String("host", "", "server host")
		port       = fs.Int("port", 0, "server port (default 25575)")
		password   = fs.String("password", "", "rcon password (prefer RCON_PASSWORD)")
		configPath = fs.String("config", defaultConfigPath(), "path to JSON config")
		server     = fs.String("server", "", "named server from the config")
		singlePkt  = fs.Bool("single-packet", false, "read one response packet per command (for servers that mishandle the multi-packet terminator)")
		drain      = fs.Bool("drain", false, "read response packets until the connection goes idle (for servers like Project Zomboid that mishandle the terminator but still split large replies)")
		showVer    = fs.Bool("version", false, "print version and exit")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *showVer {
		_, _ = fmt.Fprintf(stdout, "rcon %s (%s) built %s\n", Version, Revision, BuildDate)
		return 0
	}

	// A --config the user passed explicitly must exist; the default path is
	// allowed to be missing. fs.Visit reports only the flags actually set, so it
	// stays correct even when someone passes the default path on purpose.
	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
		}
	})
	cfg, err := LoadConfig(*configPath, explicitConfig)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	target, err := Resolve(cfg,
		Flags{Host: *host, Port: *port, Password: *password, Server: *server, SinglePacket: *singlePkt, Drain: *drain},
		envFromOS())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	var clientOpts []rconclient.Option
	switch {
	case target.Drain:
		clientOpts = append(clientOpts, rconclient.WithReadUntilIdle(0)) // 0 => default window
	case target.SinglePacket:
		clientOpts = append(clientOpts, rconclient.WithSinglePacket())
	}
	app := &App{
		Client: rconclient.New(clientOpts...),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx := context.Background()

	if cmdArgs := fs.Args(); len(cmdArgs) > 0 {
		return app.RunSingle(ctx, target, strings.Join(cmdArgs, " "))
	}
	return app.RunInteractive(ctx, target, promptFor(*server, cfg))
}

func envFromOS() Env {
	port, _ := strconv.Atoi(os.Getenv("RCON_PORT"))
	singlePacket, _ := strconv.ParseBool(os.Getenv("RCON_SINGLE_PACKET"))
	drain, _ := strconv.ParseBool(os.Getenv("RCON_DRAIN"))
	return Env{
		Host:         os.Getenv("RCON_HOST"),
		Port:         port,
		Password:     os.Getenv("RCON_PASSWORD"),
		SinglePacket: singlePacket,
		Drain:        drain,
	}
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".rcon.json"
	}
	return home + "/.rcon.json"
}

func promptFor(server string, cfg Config) string {
	if server != "" {
		return server + "> "
	}
	if cfg.Default != "" {
		return cfg.Default + "> "
	}
	return "rcon> "
}
