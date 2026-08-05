package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

// App holds the CLI's IO and client, so behavior is testable without real IO.
type App struct {
	Client *rconclient.Client
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// RunSingle executes one command and returns a process exit code.
func (a *App) RunSingle(ctx context.Context, target Resolved, command string) int {
	out, err := a.Client.Execute(ctx, target.Address, target.Password, command)
	if err != nil {
		_, _ = fmt.Fprintln(a.Stderr, "error:", err)
		return exitCode(err)
	}
	_, _ = fmt.Fprintln(a.Stdout, out)
	return 0
}

// exitCode maps an error to a process exit code.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, rcon.ErrAuthFailed):
		return 3
	default:
		return 2
	}
}
