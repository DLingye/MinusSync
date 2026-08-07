package hash

import (
	"testing"
)

func TestCompute(t *testing.T) {
	h := Compute([]byte("hello"))
	if h.IsZero() {
		t.Error("hash should not be zero")
	}
	if h.Hex() != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Errorf("unexpected hash: %s", h.Hex())
	}
}

func TestFromHex(t *testing.T) {
	h, err := FromHex("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
	if err != nil {
		t.Fatal(err)
	}
	if h.Hex() != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Error("hex roundtrip failed")
	}
}

func TestFromHexInvalid(t *testing.T) {
	_, err := FromHex("too-short")
	if err == nil {
		t.Error("expected error for short hex")
	}
}

func TestString(t *testing.T) {
	h := Compute([]byte("test"))
	s := h.String()
	if len(s) != 7 {
		t.Errorf("String() should return 7 chars, got %d", len(s))
	}
}

func TestZero(t *testing.T) {
	if !Zero.IsZero() {
		t.Error("Zero should be zero")
	}
	h := Compute([]byte("data"))
	if h.IsZero() {
		t.Error("computed hash should not be zero")
	}
}

func TestEqual(t *testing.T) {
	a := Compute([]byte("a"))
	b := Compute([]byte("a"))
	c := Compute([]byte("c"))
	if !a.Equal(b) {
		t.Error("same content should produce equal hashes")
	}
	if a.Equal(c) {
		t.Error("different content should produce different hashes")
	}
}
