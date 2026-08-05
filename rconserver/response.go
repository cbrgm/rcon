package rconserver

import (
	"io"

	"github.com/cbrgm/rcon/rcon"
)

// responseWriter accumulates a handler's response bytes. The server frames them
// into RESPONSE_VALUE packets once the handler returns.
type responseWriter struct {
	buf []byte
}

func (w *responseWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.buf = append(w.buf, s...)
	return len(s), nil
}

// writeResponse frames body into RESPONSE_VALUE packets addressed to reqID,
// splitting bodies larger than the payload cap. An empty body is written as a
// single empty packet so the client always sees a response.
func writeResponse(w io.Writer, reqID int32, body []byte) error {
	if len(body) == 0 {
		_, err := rcon.Packet{ID: reqID, Type: rcon.TypeResponseValue}.WriteTo(w)
		return err
	}
	for len(body) > 0 {
		n := min(len(body), rcon.MaxPayloadSize)
		p := rcon.Packet{ID: reqID, Type: rcon.TypeResponseValue, Body: string(body[:n])}
		if _, err := p.WriteTo(w); err != nil {
			return err
		}
		body = body[n:]
	}
	return nil
}
