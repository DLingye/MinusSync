// Package hash provides SHA-256 hashing utilities for MinusSync.
// All objects in MinusSync are content-addressed using SHA-256.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Size is the length of a SHA-256 hash in bytes.
const Size = 32

// HexSize is the length of a SHA-256 hash in hex characters.
const HexSize = 64

// Hash represents a SHA-256 hash.
type Hash [Size]byte

// Zero is the all-zero hash, used as a sentinel value.
var Zero Hash

// Compute returns the SHA-256 hash of data.
func Compute(data []byte) Hash {
	return sha256.Sum256(data)
}

// FromHex parses a hex-encoded SHA-256 hash string.
func FromHex(s string) (Hash, error) {
	var h Hash
	if len(s) != HexSize {
		return h, fmt.Errorf("invalid hash length: got %d, expected %d", len(s), HexSize)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, fmt.Errorf("invalid hex hash: %w", err)
	}
	copy(h[:], b)
	return h, nil
}

// Hex returns the hex-encoded string representation of the hash.
func (h Hash) Hex() string {
	return hex.EncodeToString(h[:])
}

// String returns a short (7-char) prefix of the hash for display.
func (h Hash) String() string {
	return h.Hex()[:7]
}

// IsZero reports whether h is the zero hash.
func (h Hash) IsZero() bool {
	return h == Zero
}

// Equal reports whether h and other are equal.
func (h Hash) Equal(other Hash) bool {
	return h == other
}

// Prefix returns the first n hex characters of the hash.
func (h Hash) Prefix(n int) string {
	s := h.Hex()
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

// DirPrefix returns the first 2 hex characters, used for object directory sharding.
func (h Hash) DirPrefix() string {
	return h.Prefix(2)
}

// Rest returns the remaining hex characters after the first 2, used as the object filename.
func (h Hash) Rest() string {
	return h.Hex()[2:]
}
