package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMissingMustExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := LoadConfig(path, true)
	if err == nil {
		t.Fatal("expected error for missing file with mustExist=true")
	}
}

func TestLoadConfigMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path, false)
	if err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("err = %v, want a wrapped parse error", err)
	}
}

func TestLoadConfigMissingOptional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	cfg, err := LoadConfig(path, false)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if cfg.Default != "" || cfg.Servers != nil {
		t.Fatalf("cfg = %+v, want empty Config", cfg)
	}
}

func TestResolvePrecedenceFlagWins(t *testing.T) {
	cfg := Config{
		Default: "mc",
		Servers: map[string]ServerConfig{"mc": {Host: "file-host", Port: 1, Password: "file-pw"}},
	}
	got, err := Resolve(cfg,
		Flags{Host: "flag-host", Port: 2, Password: "flag-pw"},
		Env{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "flag-host:2" || got.Password != "flag-pw" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveEnvOverridesFile(t *testing.T) {
	cfg := Config{Default: "mc", Servers: map[string]ServerConfig{"mc": {Host: "file-host", Port: 1, Password: "file-pw"}}}
	got, err := Resolve(cfg, Flags{Server: "mc"}, Env{Password: "env-pw"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != "env-pw" || got.Address != "file-host:1" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveNoHost(t *testing.T) {
	_, err := Resolve(Config{}, Flags{}, Env{})
	if err == nil {
		t.Fatal("expected error when no host is resolvable")
	}
}
