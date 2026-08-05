package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestPacketTypeConstants(t *testing.T) {
	cases := []struct {
		name string
		got  PacketType
		want int32
	}{
		{"ResponseValue", TypeResponseValue, 0},
		{"ExecCommand", TypeExecCommand, 2},
		{"AuthResponse", TypeAuthResponse, 2},
		{"Auth", TypeAuth, 3},
	}
	for _, c := range cases {
		if int32(c.got) != c.want {
			t.Errorf("%s = %d, want %d", c.name, int32(c.got), c.want)
		}
	}
}

func TestPacketSizeConstants(t *testing.T) {
	if MaxPayloadSize != 4096 || MinPacketSize != 10 || HeaderSize != 8 || PaddingSize != 2 {
		t.Fatalf("unexpected size constants: payload=%d min=%d header=%d pad=%d",
			MaxPayloadSize, MinPacketSize, HeaderSize, PaddingSize)
	}
}

func TestPacketWriteTo(t *testing.T) {
	p := Packet{ID: 1, Type: TypeAuth, Body: "pw"}
	var buf bytes.Buffer
	n, err := p.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	// Size = 8 (header) + 2 (body) + 2 (padding) = 12
	want := []byte{
		0x0C, 0x00, 0x00, 0x00, // Size = 12 LE
		0x01, 0x00, 0x00, 0x00, // ID = 1 LE
		0x03, 0x00, 0x00, 0x00, // Type = 3 LE
		'p', 'w', // Body
		0x00, // body NUL
		0x00, // trailing NUL
	}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded = % x, want % x", got, want)
	}
	// WriteTo reports total bytes written = 4 (size field) + Size value.
	if n != int64(4+12) {
		t.Fatalf("n = %d, want %d", n, 4+12)
	}
}

func TestPacketWriteToEmptyBody(t *testing.T) {
	p := Packet{ID: 7, Type: TypeResponseValue, Body: ""}
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 4+MinPacketSize { // 4 size field + 10
		t.Fatalf("len = %d, want %d", buf.Len(), 4+MinPacketSize)
	}
}

func TestPacketRoundTrip(t *testing.T) {
	in := Packet{ID: 42, Type: TypeExecCommand, Body: "status"}
	var buf bytes.Buffer
	if _, err := in.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	var out Packet
	n, err := out.ReadFrom(&buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if out.ID != in.ID || out.Type != in.Type || out.Body != in.Body {
		t.Fatalf("round-trip mismatch: got %+v want %+v", out, in)
	}
	if n != int64(4+HeaderSize+len(in.Body)+PaddingSize) {
		t.Fatalf("n = %d unexpected", n)
	}
}

func TestPacketReadFromTooShort(t *testing.T) {
	// Size field says 4, which is below MinPacketSize.
	raw := []byte{0x04, 0x00, 0x00, 0x00, 0, 0, 0, 0}
	var p Packet
	_, err := p.ReadFrom(bytes.NewReader(raw))
	if !errors.Is(err, ErrResponseTooShort) {
		t.Fatalf("err = %v, want ErrResponseTooShort", err)
	}
}

func TestPacketReadFromTooLong(t *testing.T) {
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, uint32(MaxPayloadSize+MinPacketSize+1))
	var p Packet
	_, err := p.ReadFrom(bytes.NewReader(raw))
	if !errors.Is(err, ErrResponseTooLong) {
		t.Fatalf("err = %v, want ErrResponseTooLong", err)
	}
}
