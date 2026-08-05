package rconserver

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cbrgm/rcon/rcon"
)

func readPackets(t *testing.T, b []byte) []rcon.Packet {
	t.Helper()
	var out []rcon.Packet
	r := bytes.NewReader(b)
	for r.Len() > 0 {
		var p rcon.Packet
		if _, err := p.ReadFrom(r); err != nil {
			t.Fatalf("ReadFrom: %v", err)
		}
		out = append(out, p)
	}
	return out
}

func TestResponseWriterImplementsWriter(t *testing.T) {
	w := &responseWriter{}
	if _, err := w.WriteString("ab"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("c")); err != nil {
		t.Fatal(err)
	}
	if string(w.buf) != "abc" {
		t.Fatalf("buf = %q", w.buf)
	}
}

func TestWriteResponseSinglePacket(t *testing.T) {
	var buf bytes.Buffer
	if err := writeResponse(&buf, 7, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	ps := readPackets(t, buf.Bytes())
	if len(ps) != 1 || ps[0].ID != 7 || ps[0].Type != rcon.TypeResponseValue || ps[0].Body != "hello" {
		t.Fatalf("packets = %+v", ps)
	}
}

func TestWriteResponseEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeResponse(&buf, 3, nil); err != nil {
		t.Fatal(err)
	}
	ps := readPackets(t, buf.Bytes())
	if len(ps) != 1 || ps[0].Body != "" || ps[0].ID != 3 {
		t.Fatalf("empty response should be one empty packet, got %+v", ps)
	}
}

func TestWriteResponseChunksOverPayloadCap(t *testing.T) {
	body := strings.Repeat("A", 2*rcon.MaxPayloadSize+50)
	var buf bytes.Buffer
	if err := writeResponse(&buf, 1, []byte(body)); err != nil {
		t.Fatal(err)
	}
	ps := readPackets(t, buf.Bytes())
	if len(ps) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(ps))
	}
	var got strings.Builder
	for _, p := range ps {
		if p.ID != 1 || p.Type != rcon.TypeResponseValue {
			t.Fatalf("bad chunk %+v", p)
		}
		if len(p.Body) > rcon.MaxPayloadSize {
			t.Fatalf("chunk exceeds cap: %d", len(p.Body))
		}
		got.WriteString(p.Body)
	}
	if got.String() != body {
		t.Fatal("reassembled body mismatch")
	}
}
