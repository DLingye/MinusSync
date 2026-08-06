package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinusSync/internal/hash"
)

// HeadRefPrefix is the prefix for symbolic HEAD references.
const HeadRefPrefix = "ref: "

// ReadHead reads the HEAD file.
// Returns the reference target (e.g., "refs/heads/main") for symbolic refs,
// or the raw hash for detached HEAD.
func (r *Refs) ReadHead(headPath string) (isSymbolic bool, target string, detachedHash hash.Hash, err error) {
	data, err := os.ReadFile(headPath)
	if err != nil {
		return false, "", hash.Zero, fmt.Errorf("read HEAD: %w", err)
	}

	content := strings.TrimSpace(string(data))

	if strings.HasPrefix(content, HeadRefPrefix) {
		return true, content[len(HeadRefPrefix):], hash.Zero, nil
	}

	h, err := hash.FromHex(content)
	if err != nil {
		return false, "", hash.Zero, fmt.Errorf("invalid HEAD content: %s", content)
	}
	return false, "", h, nil
}

// WriteHeadSymbolic writes a symbolic HEAD reference (e.g., pointing to a branch).
func (r *Refs) WriteHeadSymbolic(headPath, ref string) error {
	content := HeadRefPrefix + ref + "\n"
	return os.WriteFile(headPath, []byte(content), 0644)
}

// WriteHeadDetached writes a detached HEAD pointing directly to a commit.
func (r *Refs) WriteHeadDetached(headPath string, h hash.Hash) error {
	return os.WriteFile(headPath, []byte(h.Hex()+"\n"), 0644)
}

// ResolveHEAD resolves HEAD to a commit hash.
// For symbolic refs, it follows the chain to find the commit.
// For detached HEAD, it returns the hash directly.
func (r *Refs) ResolveHEAD(headPath string) (hash.Hash, error) {
	isSymbolic, target, detHash, err := r.ReadHead(headPath)
	if err != nil {
		return hash.Zero, err
	}

	if !isSymbolic {
		return detHash, nil
	}

	// Follow the symbolic reference
	// "refs/heads/main" → read .msync/refs/heads/main
	// r.refsDir is .msync/refs, so resolve from .msync/ parent
	refPath := filepath.Join(filepath.Dir(r.refsDir), target)
	return readRef(refPath)
}

// CurrentBranch returns the current branch name, or empty string if detached.
// The headPath should be the path to the .msync/HEAD file.
func (r *Refs) CurrentBranch(headPath string) (string, error) {
	isSymbolic, target, _, err := r.ReadHead(headPath)
	if err != nil {
		return "", err
	}

	if !isSymbolic {
		return "", nil // Detached HEAD
	}

	// Extract branch name from "refs/heads/main" or "heads/main"
	if strings.HasPrefix(target, "refs/heads/") {
		return target[len("refs/heads/"):], nil
	}
	if strings.HasPrefix(target, HeadsDir+"/") {
		return target[len(HeadsDir)+1:], nil
	}

	return target, nil
}
