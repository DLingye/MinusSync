package protocol

import (
	"github.com/MinusSync/internal/hash"
)

// RefAdvertisement contains a ref name and its hash.
type RefAdvertisement struct {
	Name string
	Hash hash.Hash
}

// EncodeRefLine encodes a ref advertisement.
func EncodeRefLine(name string, h hash.Hash) []byte {
	buf := make([]byte, 32+1+len(name))
	copy(buf[0:32], h[:])
	buf[32] = byte(len(name))
	copy(buf[33:], []byte(name))
	return buf
}

// DecodeRefLine decodes a ref advertisement from a payload.
func DecodeRefLine(payload []byte) (name string, h hash.Hash, err error) {
	if len(payload) < 33 {
		return "", hash.Zero, errInvalidPayload("REF_LINE")
	}
	copy(h[:], payload[0:32])
	nameLen := int(payload[32])
	if len(payload) < 33+nameLen {
		return "", hash.Zero, errInvalidPayload("REF_LINE")
	}
	name = string(payload[33 : 33+nameLen])
	return name, h, nil
}

// EncodeWanted encodes a wanted object hash.
func EncodeWanted(h hash.Hash) []byte {
	buf := make([]byte, 32)
	copy(buf, h[:])
	return buf
}

// DecodeWanted decodes a wanted object hash.
func DecodeWanted(payload []byte) (hash.Hash, error) {
	if len(payload) < 32 {
		return hash.Zero, errInvalidPayload("WANTED")
	}
	var h hash.Hash
	copy(h[:], payload[0:32])
	return h, nil
}

// EncodePackHeader encodes a pack header with object count.
func EncodePackHeader(count uint32) []byte {
	buf := make([]byte, 4)
	WriteUint32BE(buf, count)
	return buf
}

// EncodePackObject encodes a single pack object.
func EncodePackObject(h hash.Hash, objType uint8, compressed []byte) []byte {
	buf := make([]byte, 32+1+4+len(compressed))
	copy(buf[0:32], h[:])
	buf[32] = objType
	WriteUint32BE(buf[33:37], uint32(len(compressed)))
	copy(buf[37:], compressed)
	return buf
}

// EncodeError encodes an error message.
func EncodeError(code uint32, message string) []byte {
	msgBytes := []byte(message)
	buf := make([]byte, 4+1+len(msgBytes))
	WriteUint32BE(buf[0:4], code)
	buf[4] = byte(len(msgBytes))
	copy(buf[5:], msgBytes)
	return buf
}

// DecodeError decodes an error message.
func DecodeError(payload []byte) (code uint32, message string, err error) {
	if len(payload) < 5 {
		return 0, "", errInvalidPayload("ERROR")
	}
	code = ReadUint32BE(payload[0:4])
	msgLen := int(payload[4])
	if len(payload) < 5+msgLen {
		return 0, "", errInvalidPayload("ERROR")
	}
	message = string(payload[5 : 5+msgLen])
	return code, message, nil
}

func errInvalidPayload(msgType string) error {
	return &ProtocolError{Message: "invalid payload for " + msgType}
}

// ProtocolError is returned for protocol-level errors.
type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string {
	return "protocol error: " + e.Message
}

// WriteUint32BE writes a uint32 in big-endian to buf.
func WriteUint32BE(buf []byte, v uint32) {
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
}

// ReadUint32BE reads a uint32 in big-endian from buf.
func ReadUint32BE(buf []byte) uint32 {
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
}
