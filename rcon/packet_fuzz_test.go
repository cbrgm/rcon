package rcon

import (
	"bytes"
	"testing"
)

func FuzzPacketReadFrom(f *testing.F) {
	// Seed with a valid frame.
	var buf bytes.Buffer
	_, _ = Packet{ID: 1, Type: TypeExecCommand, Body: "ok"}.WriteTo(&buf)
	f.Add(buf.Bytes())
	f.Add([]byte{})
	f.Add([]byte{0x0A, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		var p Packet
		// Must not panic. Any error is acceptable; a successful decode must
		// round-trip back to bytes without panicking.
		if _, err := p.ReadFrom(bytes.NewReader(data)); err == nil {
			var out bytes.Buffer
			_, _ = p.WriteTo(&out)
		}
	})
}
