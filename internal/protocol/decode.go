package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Decoder reads protocol messages from a reader.
type Decoder struct {
	r io.Reader
}

// NewDecoder creates a new protocol decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Read reads the next protocol message.
func (d *Decoder) Read() (msgType uint16, payload []byte, err error) {
	// Read 4-byte length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(d.r, lenBuf); err != nil {
		return 0, nil, fmt.Errorf("read length: %w", err)
	}

	totalLen := binary.BigEndian.Uint32(lenBuf)
	if totalLen < 6 {
		return 0, nil, fmt.Errorf("invalid frame length: %d", totalLen)
	}

	// Read 2-byte type
	typeBuf := make([]byte, 2)
	if _, err := io.ReadFull(d.r, typeBuf); err != nil {
		return 0, nil, fmt.Errorf("read type: %w", err)
	}
	msgType = binary.BigEndian.Uint16(typeBuf)

	// Read payload
	payloadLen := int(totalLen) - 6
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(d.r, payload); err != nil {
			return 0, nil, fmt.Errorf("read payload: %w", err)
		}
	}

	return msgType, payload, nil
}

// ReceiveHandshake reads and verifies the protocol handshake.
func ReceiveHandshake(r io.Reader) (major, minor uint16, err error) {
	buf := make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, fmt.Errorf("read handshake: %w", err)
	}

	var magic [4]byte
	copy(magic[:], buf[0:4])
	if magic != HandshakeMagic {
		return 0, 0, fmt.Errorf("invalid handshake magic: %v", magic)
	}

	major = binary.BigEndian.Uint16(buf[4:6])
	minor = binary.BigEndian.Uint16(buf[6:8])
	return major, minor, nil
}
