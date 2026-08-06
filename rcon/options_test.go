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
	if s.mode != readMulti {
		t.Fatalf("mode should default to readMulti, got %d", s.mode)
	}
	if s.idleWindow != DefaultIdleWindow {
		t.Fatalf("idleWindow = %v, want %v", s.idleWindow, DefaultIdleWindow)
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
	if s.mode != readSingle {
		t.Fatalf("WithSinglePacket should set mode readSingle, got %d", s.mode)
	}
}

func TestWithReadUntilIdleOption(t *testing.T) {
	s := newSettings([]Option{WithReadUntilIdle(25 * time.Millisecond)})
	if s.mode != readIdle {
		t.Fatalf("mode = %d, want readIdle", s.mode)
	}
	if s.idleWindow != 25*time.Millisecond {
		t.Fatalf("idleWindow = %v, want 25ms", s.idleWindow)
	}

	// A non-positive window keeps the default.
	s = newSettings([]Option{WithReadUntilIdle(0)})
	if s.mode != readIdle || s.idleWindow != DefaultIdleWindow {
		t.Fatalf("mode/window = %d/%v, want readIdle/%v", s.mode, s.idleWindow, DefaultIdleWindow)
	}
}
