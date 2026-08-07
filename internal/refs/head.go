package refs

import (
	"fmt"
	"os"
	"strings"

	"github.com/MinusSync/internal/hash"
)

// ReadHead reads the HEAD file and returns the commit hash.
// In single-branch mode, HEAD is always a direct hash.
func (r *Refs) ReadHead(headPath string) (hash.Hash, error) {
	data, err := os.ReadFile(headPath)
	if err != nil {
		return hash.Zero, fmt.Errorf("read HEAD: %w", err)
	}
	content := strings.TrimSpace(string(data))
	h, err := hash.FromHex(content)
	if err != nil {
		return hash.Zero, fmt.Errorf("invalid HEAD content: %s", content)
	}
	return h, nil
}

// WriteHead writes HEAD pointing to a commit hash.
func (r *Refs) WriteHead(headPath string, h hash.Hash) error {
	return os.WriteFile(headPath, []byte(h.Hex()+"\n"), 0644)
}

// ResolveHEAD resolves HEAD to a commit hash.
func (r *Refs) ResolveHEAD(headPath string) (hash.Hash, error) {
	return r.ReadHead(headPath)
}
