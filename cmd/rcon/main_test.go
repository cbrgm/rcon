package main

import (
	"bytes"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	p := t.TempDir() + "/rcon.json"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func splitAddr(t *testing.T, addr string) (host, port string) {
	t.Helper()
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	return h, p
}

func TestRunVersion(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--version"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out.String(), "rcon dev (none)") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--help"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("--help code = %d", code)
	}
}

func TestRunBadFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--nope"}, nil, &out, &errBuf); code != 2 {
		t.Fatalf("bad flag code = %d", code)
	}
}

func TestRunExplicitConfigMissing(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"--config", "/nonexistent/rcon.json", "list"}, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errBuf.String(), "error") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}

func TestRunNoHost(t *testing.T) {
	cfgPath := writeTempConfig(t, "{}")
	t.Setenv("RCON_HOST", "")
	t.Setenv("RCON_PORT", "")
	t.Setenv("RCON_PASSWORD", "")
	var out, errBuf bytes.Buffer
	if code := run([]string{"--config", cfgPath, "list"}, nil, &out, &errBuf); code != 2 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
}

func TestRunSingleShot(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "3 players")
	host, port := splitAddr(t, srv.Addr())
	cfgPath := writeTempConfig(t, "{}")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"--config", cfgPath, "--host", host, "--port", port, "--password", "secret", "list",
	}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
	if strings.TrimSpace(out.String()) != "3 players" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestRunInteractiveViaMain(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("list", "ok")
	host, port := splitAddr(t, srv.Addr())
	cfgPath := writeTempConfig(t, "{}")

	in := strings.NewReader("list\n:exit\n")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"--config", cfgPath, "--host", host, "--port", port, "--password", "secret",
	}, in, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestEnvFromOS(t *testing.T) {
	t.Setenv("RCON_HOST", "h")
	t.Setenv("RCON_PORT", "42")
	t.Setenv("RCON_PASSWORD", "p")
	e := envFromOS()
	if e.Host != "h" || e.Port != 42 || e.Password != "p" {
		t.Fatalf("env = %+v", e)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	if !strings.HasSuffix(defaultConfigPath(), ".rcon.json") {
		t.Fatalf("path = %q", defaultConfigPath())
	}
}

func TestPromptFor(t *testing.T) {
	cases := []struct {
		server string
		cfg    Config
		want   string
	}{
		{"mc", Config{}, "mc> "},
		{"", Config{Default: "d"}, "d> "},
		{"", Config{}, "rcon> "},
	}
	for _, c := range cases {
		if got := promptFor(c.server, c.cfg); got != c.want {
			t.Errorf("promptFor(%q, %+v) = %q, want %q", c.server, c.cfg, got, c.want)
		}
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("nil -> %d, want 0", got)
	}
	if got := exitCode(rcon.ErrAuthFailed); got != 3 {
		t.Errorf("ErrAuthFailed -> %d, want 3", got)
	}
	if got := exitCode(errors.New("boom")); got != 2 {
		t.Errorf("other -> %d, want 2", got)
	}
}

func TestRunInteractiveDialError(t *testing.T) {
	var out, errBuf bytes.Buffer
	app := &App{Client: rconclient.New(), Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errBuf}
	code := app.RunInteractive(t.Context(), Resolved{Address: "127.0.0.1:1", Password: "x"}, "")
	if code == 0 {
		t.Fatal("expected a non-zero code on dial failure")
	}
}

func TestRunInteractiveEmptyLineAndCommandError(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	srv.Handle("good", "ok")
	// blank line -> continue; an over-long command -> per-command error printed to
	// stderr, loop continues; good -> reply; :exit -> return 0.
	longCmd := strings.Repeat("x", 1500)
	in := strings.NewReader("\n" + longCmd + "\ngood\n:exit\n")
	var out, errBuf bytes.Buffer
	app := &App{Client: rconclient.New(), Stdin: in, Stdout: &out, Stderr: &errBuf}
	code := app.RunInteractive(t.Context(), Resolved{Address: srv.Addr(), Password: "secret"}, "rcon> ")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "error") {
		t.Fatalf("expected the over-long command to print an error; stderr = %q", errBuf.String())
	}
}
