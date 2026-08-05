package rcon

import "io"

// authenticate performs the RCON auth handshake over rw. It writes a
// TypeAuth packet with the password, tolerates an optional empty
// TypeResponseValue sentinel some servers send first, then reads the
// TypeAuthResponse. An ID of -1 in the auth response means the password was
// rejected.
func authenticate(rw io.ReadWriter, password string, nextID func() int32) error {
	reqID := nextID()
	if _, err := (Packet{ID: reqID, Type: TypeAuth, Body: password}).WriteTo(rw); err != nil {
		return err
	}
	var resp Packet
	if _, err := resp.ReadFrom(rw); err != nil {
		return err
	}
	// Some servers send an empty RESPONSE_VALUE before the auth response.
	if resp.Type == TypeResponseValue {
		if _, err := resp.ReadFrom(rw); err != nil {
			return err
		}
	}
	if resp.Type != TypeAuthResponse {
		return ErrInvalidAuthResponse
	}
	if resp.ID == -1 {
		return ErrAuthFailed
	}
	return nil
}
