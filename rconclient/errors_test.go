package rconclient_test

import (
	"errors"
	"testing"

	"github.com/cbrgm/rcon/internal/fakercon"
	"github.com/cbrgm/rcon/rcon"
	"github.com/cbrgm/rcon/rconclient"
)

func TestErrorSentinelsAliasCore(t *testing.T) {
	cases := []struct {
		name       string
		reexported error
		core       error
	}{
		{"ErrAuthFailed", rconclient.ErrAuthFailed, rcon.ErrAuthFailed},
		{"ErrCommandEmpty", rconclient.ErrCommandEmpty, rcon.ErrCommandEmpty},
		{"ErrCommandTooLong", rconclient.ErrCommandTooLong, rcon.ErrCommandTooLong},
	}
	for _, c := range cases {
		if !errors.Is(c.reexported, c.core) {
			t.Errorf("rconclient.%s should match rcon.%s", c.name, c.name)
		}
	}
}

func TestExecuteAuthFailureMatchesReexport(t *testing.T) {
	srv := fakercon.Start(t, "secret")
	_, err := rconclient.Execute(t.Context(), srv.Addr(), "wrong", "list")
	if !errors.Is(err, rconclient.ErrAuthFailed) {
		t.Fatalf("err = %v, want rconclient.ErrAuthFailed", err)
	}
}
