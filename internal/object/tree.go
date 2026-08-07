package object

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MinusSync/internal/hash"
)

// File mode constants.
const (
	ModeRegular  uint32 = 0100644
	ModeExec     uint32 = 0100755
	ModeSymlink  uint32 = 0120000
	ModeDir      uint32 = 0040000
	ModeSubmod   uint32 = 0160000
)

// TreeEntry represents one entry in a tree object.
type TreeEntry struct {
	Mode uint32
	Name string
	Hash hash.Hash
	Type ObjectType // blob or tree
}

// SerializeTree converts tree entries to the on-disk format.
// Format: "<mode> <name>\0<20-byte-hash>" for each entry, sorted by name.
func SerializeTree(entries []TreeEntry) []byte {
	// Sort entries by name for deterministic hashing
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	var buf bytes.Buffer
	for _, e := range entries {
		// mode and name followed by null
		fmt.Fprintf(&buf, "%o %s", e.Mode, e.Name)
		buf.WriteByte(0)
		// raw 32-byte hash
		buf.Write(e.Hash[:])
	}
	return buf.Bytes()
}

// ParseTree parses serialized tree data into entries.
func ParseTree(data []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	pos := 0

	for pos < len(data) {
		// Find null byte separating "mode name" from hash
		nullIdx := bytes.IndexByte(data[pos:], 0)
		if nullIdx == -1 {
			return nil, fmt.Errorf("malformed tree: missing null byte at position %d", pos)
		}
		nullIdx += pos

		header := string(data[pos:nullIdx])
		pos = nullIdx + 1

		// Parse mode and name
		spaceIdx := strings.IndexByte(header, ' ')
		if spaceIdx == -1 {
			return nil, fmt.Errorf("malformed tree entry: %q", header)
		}

		var mode uint32
		if _, err := fmt.Sscanf(header[:spaceIdx], "%o", &mode); err != nil {
			return nil, fmt.Errorf("invalid mode in tree entry %q: %w", header, err)
		}
		name := header[spaceIdx+1:]

		// Read 32-byte hash
		if pos+hash.Size > len(data) {
			return nil, fmt.Errorf("malformed tree: unexpected end at position %d", pos)
		}
		var h hash.Hash
		copy(h[:], data[pos:pos+hash.Size])
		pos += hash.Size

		// Determine entry type
		entryType := TypeBlob
		if mode == ModeDir {
			entryType = TypeTree
		}

		entries = append(entries, TreeEntry{
			Mode: mode,
			Name: name,
			Hash: h,
			Type: entryType,
		})
	}

	return entries, nil
}

// BuildTree walks a directory and builds tree objects recursively.
// Returns the hash of the root tree.
func BuildTree(objectsDir, dirPath string, ignores func(string) bool) (hash.Hash, error) {
	return buildTreeRecursive(objectsDir, dirPath, ignores)
}

func buildTreeRecursive(objectsDir, dirPath string, ignores func(string) bool) (hash.Hash, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return hash.Zero, err
	}

	var treeEntries []TreeEntry

	for _, entry := range entries {
		name := entry.Name()

		// Skip .msync directory
		if name == ".msync" {
			continue
		}

		// Check ignore patterns
		fullPath := filepath.Join(dirPath, name)
		if ignores != nil && ignores(fullPath) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return hash.Zero, err
		}

		if entry.IsDir() {
			// Recurse into subdirectory
			subTreeHash, err := buildTreeRecursive(objectsDir, fullPath, ignores)
			if err != nil {
				return hash.Zero, err
			}
			treeEntries = append(treeEntries, TreeEntry{
				Mode: ModeDir,
				Name: name,
				Hash: subTreeHash,
				Type: TypeTree,
			})
		} else if info.Mode()&os.ModeSymlink != 0 {
			// Handle symlinks
			target, err := os.Readlink(fullPath)
			if err != nil {
				return hash.Zero, err
			}
			h, err := WriteBlob(objectsDir, []byte(target))
			if err != nil {
				return hash.Zero, err
			}
			treeEntries = append(treeEntries, TreeEntry{
				Mode: ModeSymlink,
				Name: name,
				Hash: h,
				Type: TypeBlob,
			})
		} else if info.Mode().IsRegular() {
			// Regular file
			mode := ModeRegular
			if info.Mode()&0111 != 0 {
				mode = ModeExec
			}

			h, err := CreateBlob(objectsDir, fullPath)
			if err != nil {
				return hash.Zero, err
			}
			treeEntries = append(treeEntries, TreeEntry{
				Mode: mode,
				Name: name,
				Hash: h,
				Type: TypeBlob,
			})
		}
	}

	treeData := SerializeTree(treeEntries)
	return Write(objectsDir, Format(TypeTree, treeData))
}

// CheckoutTree writes the contents of a tree to the filesystem.
func CheckoutTree(objectsDir string, treeHash hash.Hash, targetDir string) error {
	// Read tree object
	data, err := Read(objectsDir, treeHash)
	if err != nil {
		return err
	}

	header, offset, err := ParseHeader(data)
	if err != nil {
		return err
	}
	if header.Type != TypeTree {
		return &ErrUnexpectedType{Got: header.Type, Want: TypeTree}
	}

	entries, err := ParseTree(data[offset:])
	if err != nil {
		return err
	}

	for _, entry := range entries {
		target := filepath.Join(targetDir, entry.Name)

		if entry.Type == TypeTree {
			// Create directory and recurse
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			if err := CheckoutTree(objectsDir, entry.Hash, target); err != nil {
				return err
			}
		} else {
			// Write file
			content, err := ReadBlob(objectsDir, entry.Hash)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			mode := os.FileMode(0644)
			if entry.Mode == ModeExec {
				mode = 0755
			} else if entry.Mode == ModeSymlink {
				// Symlink target is stored as blob content
				if err := os.Symlink(string(content), target); err != nil {
					return err
				}
				continue
			}

			if err := os.WriteFile(target, content, mode); err != nil {
				return err
			}
		}
	}

	// Remove files that exist in targetDir but not in tree
	return cleanDir(objectsDir, treeHash, targetDir)
}

// cleanDir removes files in targetDir that aren't in the tree.
func cleanDir(objectsDir string, treeHash hash.Hash, targetDir string) error {
	data, err := Read(objectsDir, treeHash)
	if err != nil {
		return err
	}

	_, offset, err := ParseHeader(data)
	if err != nil {
		return err
	}
	entries, err := ParseTree(data[offset:])
	if err != nil {
		return err
	}

	treeFiles := make(map[string]bool)
	for _, e := range entries {
		treeFiles[e.Name] = true
	}

	dirEntries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		name := entry.Name()
		if name == ".msync" {
			continue
		}
		if !treeFiles[name] {
			if err := os.RemoveAll(filepath.Join(targetDir, name)); err != nil {
				return err
			}
		}
	}

	return nil
}

// ReadTree reads a tree object and returns its entries.
func ReadTree(objectsDir string, h hash.Hash) ([]TreeEntry, error) {
	content, header, err := ReadContent(objectsDir, h)
	if err != nil {
		return nil, err
	}
	if header.Type != TypeTree {
		return nil, &ErrUnexpectedType{Got: header.Type, Want: TypeTree}
	}
	return ParseTree(content)
}

// WalkTree recursively walks a tree, calling fn for each blob entry.
func WalkTree(objectsDir string, treeHash hash.Hash, fn func(path string, entry TreeEntry) error) error {
	return walkTreeRecursive(objectsDir, treeHash, "", fn)
}

func walkTreeRecursive(objectsDir string, treeHash hash.Hash, prefix string, fn func(string, TreeEntry) error) error {
	entries, err := ReadTree(objectsDir, treeHash)
	if err != nil {
		return err
	}

	for _, e := range entries {
		path := filepath.Join(prefix, e.Name)
		if e.Type == TypeTree {
			if err := walkTreeRecursive(objectsDir, e.Hash, path, fn); err != nil {
				return err
			}
		} else {
			if err := fn(path, e); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListTree returns all file paths in a tree (non-recursive).
func ListTree(objectsDir string, treeHash hash.Hash) ([]string, map[string]TreeEntry, error) {
	entries, err := ReadTree(objectsDir, treeHash)
	if err != nil {
		return nil, nil, err
	}

	paths := make([]string, 0, len(entries))
	entryMap := make(map[string]TreeEntry, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Name)
		entryMap[e.Name] = e
	}
	return paths, entryMap, nil
}

// Ensure io is used (for future extensibility).
var _ io.Writer
