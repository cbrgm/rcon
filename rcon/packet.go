package rcon

import (
	"encoding/binary"
	"io"
)

// PacketType is the Source RCON packet type field (a little-endian int32 on the
// wire).
type PacketType int32

// Source RCON packet types. Note that TypeAuthResponse and TypeExecCommand share
// the wire value 2; they are disambiguated by direction (client sends
// TypeExecCommand, the server replies with TypeAuthResponse during auth).
const (
	// TypeResponseValue is a server response to an executed command, and the
	// type of the empty sentinel packet used to terminate multi-packet reads.
	TypeResponseValue PacketType = 0
	// TypeExecCommand is a client request to run a command.
	TypeExecCommand PacketType = 2
	// TypeAuthResponse is the server's reply to an authentication request.
	TypeAuthResponse PacketType = 2
	// TypeAuth is a client authentication request carrying the password.
	TypeAuth PacketType = 3
)

// Wire-format sizes, in bytes.
const (
	// MaxPayloadSize is the maximum RCON body payload per packet.
	MaxPayloadSize = 4096
	// HeaderSize is the size of the ID and Type fields together.
	HeaderSize = 8
	// PaddingSize is the two trailing NUL terminators.
	PaddingSize = 2
	// MinPacketSize is the smallest legal value of the Size field (empty body).
	MinPacketSize = HeaderSize + PaddingSize
)

// Packet is a single RCON protocol frame.
type Packet struct {
	// ID is the request identifier, echoed by the server in the matching reply.
	ID int32
	// Type is the packet type.
	Type PacketType
	// Body is the ASCII payload (a command, a password, or a response chunk).
	Body string
}

// WriteTo encodes the packet as a single RCON frame and writes it to w. It
// implements io.WriterTo. The returned count includes the 4-byte size field.
func (p Packet) WriteTo(w io.Writer) (int64, error) {
	size := HeaderSize + len(p.Body) + PaddingSize
	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(p.ID))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(p.Type))
	copy(buf[12:], p.Body)
	// buf already zero-filled, so the two trailing NULs are in place.
	n, err := w.Write(buf)
	return int64(n), err
}

// ReadFrom decodes a single RCON frame from r into p. It implements
// io.ReaderFrom. The returned count includes the 4-byte size field. A frame
// whose declared size is out of range yields ErrResponseTooShort or
// ErrResponseTooLong without consuming the (untrusted) body.
func (p *Packet) ReadFrom(r io.Reader) (int64, error) {
	var sizeField [4]byte
	if _, err := io.ReadFull(r, sizeField[:]); err != nil {
		return 0, err
	}
	// Compare the raw uint32 against the max before narrowing to int, so an
	// oversized frame is not mislabeled on 32-bit builds where a declared size
	// >= 0x80000000 would become a negative int.
	sizeU := binary.LittleEndian.Uint32(sizeField[:])
	if sizeU > MaxPayloadSize+MinPacketSize {
		return 4, ErrResponseTooLong
	}
	size := int(sizeU)
	if size < MinPacketSize {
		return 4, ErrResponseTooShort
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return 4, err
	}
	p.ID = int32(binary.LittleEndian.Uint32(body[0:4]))
	p.Type = PacketType(int32(binary.LittleEndian.Uint32(body[4:8])))
	// body[8:size-2] is the payload; the last two bytes are NUL padding.
	p.Body = string(body[8 : size-PaddingSize])
	return int64(4 + size), nil
}
