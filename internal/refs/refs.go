// Package refs provides reference management for branches, tags, and remotes.
package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/util"
)

// Directory names within .msync/refs/
const (
	HeadsDir   = "heads"
	TagsDir    = "tags"
	RemotesDir = "remotes"
)

// Refs manages references within a repository.
type Refs struct {
	refsDir string
}

// New creates a new Refs manager.
func New(refsDir string) *Refs {
	return &Refs{refsDir: refsDir}
}

// readRef reads a reference file and returns the hash it points to.
func readRef(path string) (hash.Hash, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hash.Zero, fmt.Errorf("ref not found: %s", path)
		}
		return hash.Zero, err
	}
	return hash.FromHex(strings.TrimSpace(string(data)))
}

// writeRef writes a hash to a reference file.
func writeRef(path string, h hash.Hash) error {
	dir := filepath.Dir(path)
	if err := util.MkdirAll(dir); err != nil {
		return err
	}
	return util.WriteFile(path, []byte(h.Hex()+"\n"), 0644)
}

// GetBranch returns the hash for a branch.
func (r *Refs) GetBranch(name string) (hash.Hash, error) {
	return readRef(filepath.Join(r.refsDir, HeadsDir, name))
}

// SetBranch sets the hash for a branch.
func (r *Refs) SetBranch(name string, h hash.Hash) error {
	return writeRef(filepath.Join(r.refsDir, HeadsDir, name), h)
}

// DeleteBranch deletes a branch reference.
func (r *Refs) DeleteBranch(name string) error {
	path := filepath.Join(r.refsDir, HeadsDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("branch %q not found", name)
	}
	return os.Remove(path)
}

// ListBranches returns all branch names.
func (r *Refs) ListBranches() ([]string, error) {
	return listRefs(filepath.Join(r.refsDir, HeadsDir))
}

// HasBranch reports whether a branch exists.
func (r *Refs) HasBranch(name string) bool {
	_, err := os.Stat(filepath.Join(r.refsDir, HeadsDir, name))
	return err == nil
}

// GetTag returns the hash for a tag (lightweight or annotated).
func (r *Refs) GetTag(name string) (hash.Hash, error) {
	return readRef(filepath.Join(r.refsDir, TagsDir, name))
}

// SetTag creates a lightweight tag.
func (r *Refs) SetTag(name string, h hash.Hash) error {
	return writeRef(filepath.Join(r.refsDir, TagsDir, name), h)
}

// DeleteTag deletes a tag reference.
func (r *Refs) DeleteTag(name string) error {
	path := filepath.Join(r.refsDir, TagsDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("tag %q not found", name)
	}
	return os.Remove(path)
}

// ListTags returns all tag names.
func (r *Refs) ListTags() ([]string, error) {
	return listRefs(filepath.Join(r.refsDir, TagsDir))
}

// GetRemoteRef returns the hash for a remote tracking branch.
func (r *Refs) GetRemoteRef(remote, branch string) (hash.Hash, error) {
	return readRef(filepath.Join(r.refsDir, RemotesDir, remote, branch))
}

// SetRemoteRef sets the hash for a remote tracking branch.
func (r *Refs) SetRemoteRef(remote, branch string, h hash.Hash) error {
	return writeRef(filepath.Join(r.refsDir, RemotesDir, remote, branch), h)
}

// ListRemotes returns all remote names.
func (r *Refs) ListRemotes() ([]string, error) {
	path := filepath.Join(r.refsDir, RemotesDir)
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var remotes []string
	for _, e := range entries {
		if e.IsDir() {
			remotes = append(remotes, e.Name())
		}
	}
	return remotes, nil
}

// ListRemoteBranches returns all branch names tracked for a remote.
func (r *Refs) ListRemoteBranches(remote string) ([]string, error) {
	return listRefs(filepath.Join(r.refsDir, RemotesDir, remote))
}

// DeleteRemoteRef deletes all tracking refs for a remote.
func (r *Refs) DeleteRemoteRef(remote string) error {
	path := filepath.Join(r.refsDir, RemotesDir, remote)
	return os.RemoveAll(path)
}

// listRefs walks a directory and returns all filenames found recursively.
func listRefs(dir string) ([]string, error) {
	var refs []string
	err := util.WalkDir(dir, func(path string, info os.FileInfo) error {
		if info.IsDir() {
			return nil
		}
		rel, err := util.RelPath(dir, path)
		if err != nil {
			return err
		}
		refs = append(refs, util.NormalizePath(rel))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return refs, nil
}
