package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Encoder writes protocol messages to a writer.
type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new protocol encoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Write sends a protocol message.
func (e *Encoder) Write(msgType uint16, payload []byte) error {
	// Frame: [4: total length][2: type][N: payload]
	totalLen := 6 + len(payload) // 4 for length itself + 2 for type

	header := make([]byte, 6)
	binary.BigEndian.PutUint32(header[0:4], uint32(totalLen))
	binary.BigEndian.PutUint16(header[4:6], msgType)

	if _, err := e.w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := e.w.Write(payload); err != nil {
			return fmt.Errorf("write payload: %w", err)
		}
	}
	return nil
}

// WriteEmpty sends a message with no payload.
func (e *Encoder) WriteEmpty(msgType uint16) error {
	return e.Write(msgType, nil)
}

// WriteUint32 writes a uint32 value as payload.
func (e *Encoder) WriteUint32(msgType uint16, v uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return e.Write(msgType, buf)
}

// WriteBytes writes a message with the given bytes as payload.
func (e *Encoder) WriteBytes(msgType uint16, data []byte) error {
	return e.Write(msgType, data)
}

// SendHandshake sends the protocol handshake.
func (e *Encoder) SendHandshake() error {
	buf := make([]byte, 8)
	copy(buf[0:4], HandshakeMagic[:])
	binary.BigEndian.PutUint16(buf[4:6], VersionMajor)
	binary.BigEndian.PutUint16(buf[6:8], VersionMinor)
	_, err := e.w.Write(buf)
	return err
}
