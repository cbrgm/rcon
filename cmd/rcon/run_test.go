package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rconclient"
)

func TestRunSingleSuccess(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	var out, errBuf bytes.Buffer
	app := &App{Client: rconclient.New(), Stdout: &out, Stderr: &errBuf}
	code := app.RunSingle(t.Context(),
		Resolved{Address: srv.Addr(), Password: "secret"}, "list")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
	if got := out.String(); got != "3 players\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunSingleAuthFailure(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	var out, errBuf bytes.Buffer
	app := &App{Client: rconclient.New(), Stdout: &out, Stderr: &errBuf}
	code := app.RunSingle(t.Context(),
		Resolved{Address: srv.Addr(), Password: "wrong"}, "list")
	if code == 0 {
		t.Fatal("expected non-zero exit on auth failure")
	}
}

func TestRunInteractive(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("a", "resA")
	srv.Handle("b", "resB")
	in := strings.NewReader("a\nb\n:exit\n")
	var out, errBuf bytes.Buffer
	app := &App{Client: rconclient.New(), Stdin: in, Stdout: &out, Stderr: &errBuf}
	code := app.RunInteractive(t.Context(),
		Resolved{Address: srv.Addr(), Password: "secret"}, "")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "resA") || !strings.Contains(got, "resB") {
		t.Fatalf("stdout missing responses: %q", got)
	}
}
