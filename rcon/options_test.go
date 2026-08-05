package rcon

import (
	"testing"
	"time"
)

func TestNewSettingsDefaults(t *testing.T) {
	s := newSettings(nil)
	if s.dialTimeout != DefaultDialTimeout || s.deadline != DefaultDeadline {
		t.Fatalf("bad timeouts: %+v", s)
	}
	if s.maxCommandLen != DefaultMaxCommandLen {
		t.Fatalf("maxCommandLen = %d", s.maxCommandLen)
	}
	if !s.multiPacket {
		t.Fatal("multiPacket should default to true")
	}
}

func TestNewSettingsOptions(t *testing.T) {
	s := newSettings([]Option{
		WithDialTimeout(2 * time.Second),
		WithDeadline(3 * time.Second),
		WithMaxCommandLen(50),
		WithSinglePacket(),
	})
	if s.dialTimeout != 2*time.Second || s.deadline != 3*time.Second {
		t.Fatalf("bad timeouts: %+v", s)
	}
	if s.maxCommandLen != 50 {
		t.Fatalf("maxCommandLen = %d", s.maxCommandLen)
	}
	if s.multiPacket {
		t.Fatal("WithSinglePacket should disable multiPacket")
	}
}
