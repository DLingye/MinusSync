// Package object provides the content-addressable object store for MinusSync.
// Objects are stored on disk as loose files under .msync/objects/XX/XXXX...
// Each object is serialized as: "<type> <size>\0<content>"
// The SHA-256 hash of the entire serialized form determines the storage path.
package object

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
)

// ObjectType identifies the kind of object.
type ObjectType uint8

const (
	TypeBlob   ObjectType = 0
	TypeTree   ObjectType = 1
	TypeCommit ObjectType = 2
	TypeTag    ObjectType = 3
)

// String returns a human-readable name for the object type.
func (t ObjectType) String() string {
	switch t {
	case TypeBlob:
		return "blob"
	case TypeTree:
		return "tree"
	case TypeCommit:
		return "commit"
	case TypeTag:
		return "tag"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// Header contains the parsed type and size from an object header.
type Header struct {
	Type ObjectType
	Size uint64
}

// typeNameToType maps string names to ObjectType.
var typeNameToType = map[string]ObjectType{
	"blob":   TypeBlob,
	"tree":   TypeTree,
	"commit": TypeCommit,
	"tag":    TypeTag,
}

// ParseHeader parses the "<type> <size>\0" header from serialized object data.
// Returns the header and the offset to the content (after the null byte).
func ParseHeader(data []byte) (*Header, int, error) {
	nullIdx := bytes.IndexByte(data, '\x00')
	if nullIdx == -1 {
		return nil, 0, fmt.Errorf("missing null byte in object header")
	}

	header := string(data[:nullIdx])
	var typeName string
	var size uint64
	if _, err := fmt.Sscanf(header, "%s %d", &typeName, &size); err != nil {
		return nil, 0, fmt.Errorf("malformed object header %q: %w", header, err)
	}

	t, ok := typeNameToType[typeName]
	if !ok {
		return nil, 0, fmt.Errorf("unknown object type %q", typeName)
	}

	return &Header{Type: t, Size: size}, nullIdx + 1, nil
}

// Format serializes an object header and content into the on-disk format.
func Format(t ObjectType, content []byte) []byte {
	header := fmt.Sprintf("%s %d\x00", t.String(), len(content))
	result := make([]byte, 0, len(header)+len(content))
	result = append(result, []byte(header)...)
	result = append(result, content...)
	return result
}

// objectPath returns the filesystem path for an object hash.
func objectPath(objectsDir string, h hash.Hash) string {
	return filepath.Join(objectsDir, h.DirPrefix(), h.Rest())
}

// Write stores raw serialized object data and returns its hash.
// The data should already be in the format "<type> <size>\0<content>".
// If the object already exists, it is not written again (deduplication).
func Write(objectsDir string, data []byte) (hash.Hash, error) {
	h := hash.Compute(data)

	path := objectPath(objectsDir, h)

	// If already exists, skip
	if _, err := os.Stat(path); err == nil {
		return h, nil
	}

	// Ensure the prefix directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return hash.Zero, fmt.Errorf("create object dir: %w", err)
	}

	// Write atomically via a temp file
	tmp, err := os.CreateTemp(dir, ".tmp-obj-*")
	if err != nil {
		return hash.Zero, fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return hash.Zero, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return hash.Zero, err
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		// If rename fails due to existing file (race), that's fine
		if os.IsExist(err) {
			return h, nil
		}
		return hash.Zero, fmt.Errorf("rename temp to object: %w", err)
	}

	return h, nil
}

// Exists reports whether an object exists in the object store.
func Exists(objectsDir string, h hash.Hash) bool {
	_, err := os.Stat(objectPath(objectsDir, h))
	return err == nil
}

// ReadRaw reads the raw serialized object data from disk.
func ReadRaw(objectsDir string, h hash.Hash) ([]byte, error) {
	path := objectPath(objectsDir, h)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object %s not found", h.Hex())
		}
		return nil, err
	}
	return data, nil
}

// Read reads and verifies an object from the store.
// It checks that the SHA-256 of the stored data matches the requested hash.
func Read(objectsDir string, h hash.Hash) ([]byte, error) {
	data, err := ReadRaw(objectsDir, h)
	if err != nil {
		return nil, err
	}

	actual := hash.Compute(data)
	if !actual.Equal(h) {
		return nil, fmt.Errorf("object %s is corrupted (hash mismatch)", h.Hex())
	}

	return data, nil
}

// ReadContent reads an object and returns the content portion (after the header).
func ReadContent(objectsDir string, h hash.Hash) ([]byte, *Header, error) {
	data, err := Read(objectsDir, h)
	if err != nil {
		return nil, nil, err
	}

	header, offset, err := ParseHeader(data)
	if err != nil {
		return nil, nil, err
	}

	if uint64(len(data)-offset) != header.Size {
		return nil, nil, fmt.Errorf("object %s size mismatch: header says %d, got %d",
			h.Hex(), header.Size, len(data)-offset)
	}

	return data[offset:], header, nil
}

// ReadHeader reads just the header of an object (useful for type checking).
func ReadHeader(objectsDir string, h hash.Hash) (*Header, error) {
	data, err := Read(objectsDir, h)
	if err != nil {
		return nil, err
	}

	header, _, err := ParseHeader(data)
	return header, err
}

// WriteBlob creates and stores a blob object.
func WriteBlob(objectsDir string, content []byte) (hash.Hash, error) {
	data := Format(TypeBlob, content)
	return Write(objectsDir, data)
}

// WriteTree creates and stores a tree object from entries.
func WriteTree(objectsDir string, entries []TreeEntry) (hash.Hash, error) {
	content := SerializeTree(entries)
	data := Format(TypeTree, content)
	return Write(objectsDir, data)
}

// WriteCommit creates and stores a commit object.
func WriteCommit(objectsDir string, c *CommitData) (hash.Hash, error) {
	content := SerializeCommit(c)
	data := Format(TypeCommit, content)
	return Write(objectsDir, data)
}

// WriteTag creates and stores a tag object.
func WriteTag(objectsDir string, t *TagData) (hash.Hash, error) {
	content := SerializeTag(t)
	data := Format(TypeTag, content)
	return Write(objectsDir, data)
}

// WriteUint32 writes a uint32 in big-endian format.
func WriteUint32(buf []byte, v uint32) {
	binary.BigEndian.PutUint32(buf, v)
}

// ReadUint32 reads a uint32 in big-endian format.
func ReadUint32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}
