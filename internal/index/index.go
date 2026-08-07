// Package index provides the staging area index for MinusSync.
// The index tracks file state between the working tree and the object store,
// enabling fast status checks without rehashing all files.
package index

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"sort"

	"github.com/MinusSync/internal/hash"
)

// Magic bytes for the index file.
var indexMagic = [4]byte{'M', 'S', 'Y', 'N'}

const indexVersion uint16 = 1

// Flag bits for IndexEntry.
const (
	FlagAssumeUnchanged = 0x0001
	FlagIntentToAdd     = 0x0002
	FlagSkipWorktree    = 0x0004
)

// IndexEntry represents one entry in the staging area.
type IndexEntry struct {
	Path    string
	Hash    hash.Hash
	Size    int64
	MtimeNs int64
	Mode    uint32
	Flags   uint16
}

// Index holds the staging area.
type Index struct {
	path    string
	Entries []IndexEntry
}

// New creates an empty index.
func New(path string) *Index {
	return &Index{
		path:    path,
		Entries: nil,
	}
}

// Load reads the index from disk.
func Load(path string) (*Index, error) {
	idx := &Index{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, err
	}

	if len(data) < 10 {
		return idx, nil // Too short to be valid
	}

	// Verify magic
	if !bytes.Equal(data[0:4], indexMagic[:]) {
		return idx, nil // Wrong magic, treat as empty
	}

	// Read version
	version := binary.BigEndian.Uint16(data[4:6])
	if version != indexVersion {
		return idx, nil // Unknown version, treat as empty
	}

	// Read entry count
	count := binary.BigEndian.Uint32(data[6:10])

	entries := make([]IndexEntry, 0, count)
	pos := 10

	for i := uint32(0); i < count; i++ {
		if pos+2 > len(data) {
			break
		}

		// Read path length
		pathLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+pathLen > len(data) {
			break
		}

		// Read path
		path := string(data[pos : pos+pathLen])
		pos += pathLen

		// Read hash
		if pos+hash.Size > len(data) {
			break
		}
		var h hash.Hash
		copy(h[:], data[pos:pos+hash.Size])
		pos += hash.Size

		// Read size
		if pos+8 > len(data) {
			break
		}
		size := int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		pos += 8

		// Read mtime
		if pos+8 > len(data) {
			break
		}
		mtime := int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		pos += 8

		// Read mode
		if pos+4 > len(data) {
			break
		}
		mode := binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4

		// Read flags
		if pos+2 > len(data) {
			break
		}
		flags := binary.BigEndian.Uint16(data[pos : pos+2])
		pos += 2

		entries = append(entries, IndexEntry{
			Path:    path,
			Hash:    h,
			Size:    size,
			MtimeNs: mtime,
			Mode:    mode,
			Flags:   flags,
		})
	}

	// Sort entries by path for binary search
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	idx.Entries = entries
	return idx, nil
}

// Save writes the index to disk.
func (idx *Index) Save() error {
	// Sort entries by path
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Path < idx.Entries[j].Path
	})

	var buf bytes.Buffer

	// Magic
	buf.Write(indexMagic[:])

	// Version
	versionBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(versionBytes, indexVersion)
	buf.Write(versionBytes)

	// Entry count
	countBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(countBytes, uint32(len(idx.Entries)))
	buf.Write(countBytes)

	// Entries
	for _, e := range idx.Entries {
		// Path length and path
		pathLen := make([]byte, 2)
		binary.BigEndian.PutUint16(pathLen, uint16(len(e.Path)))
		buf.Write(pathLen)
		buf.Write([]byte(e.Path))

		// Hash
		buf.Write(e.Hash[:])

		// Size
		sizeBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(sizeBytes, uint64(e.Size))
		buf.Write(sizeBytes)

		// Mtime
		mtimeBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(mtimeBytes, uint64(e.MtimeNs))
		buf.Write(mtimeBytes)

		// Mode
		modeBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(modeBytes, e.Mode)
		buf.Write(modeBytes)

		// Flags
		flagBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(flagBytes, e.Flags)
		buf.Write(flagBytes)
	}

	// CRC32 of all entry data
	crc := crc32.ChecksumIEEE(buf.Bytes()[10:])
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc)
	buf.Write(crcBytes)

	return os.WriteFile(idx.path, buf.Bytes(), 0644)
}

// Find looks up an entry by path. Returns nil if not found.
func (idx *Index) Find(path string) *IndexEntry {
	// Binary search
	i := sort.Search(len(idx.Entries), func(i int) bool {
		return idx.Entries[i].Path >= path
	})
	if i < len(idx.Entries) && idx.Entries[i].Path == path {
		return &idx.Entries[i]
	}
	return nil
}

// Add adds or updates an entry in the index.
func (idx *Index) Add(entry IndexEntry) {
	// Check if already exists
	i := sort.Search(len(idx.Entries), func(i int) bool {
		return idx.Entries[i].Path >= entry.Path
	})

	if i < len(idx.Entries) && idx.Entries[i].Path == entry.Path {
		idx.Entries[i] = entry
	} else {
		// Insert at position i
		idx.Entries = append(idx.Entries, IndexEntry{})
		copy(idx.Entries[i+1:], idx.Entries[i:])
		idx.Entries[i] = entry
	}
}

// Remove removes an entry from the index by path.
func (idx *Index) Remove(path string) {
	i := sort.Search(len(idx.Entries), func(i int) bool {
		return idx.Entries[i].Path >= path
	})
	if i < len(idx.Entries) && idx.Entries[i].Path == path {
		idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
	}
}

// Clear removes all entries.
func (idx *Index) Clear() {
	idx.Entries = nil
}

// Len returns the number of entries in the index.
func (idx *Index) Len() int {
	return len(idx.Entries)
}

// Dirty reports whether an entry has changed relative to the working tree.
func (idx *Index) Dirty(path string, info os.FileInfo) (bool, error) {
	entry := idx.Find(path)
	if entry == nil {
		return true, nil // Not in index = new file
	}

	// Compare stat info
	if entry.Size != info.Size() {
		return true, nil
	}

	mtimeNs := info.ModTime().UnixNano()
	if entry.MtimeNs != mtimeNs {
		return true, nil
	}

	if entry.Mode != uint32(info.Mode()) {
		return true, nil
	}

	return false, nil
}

// IsTracked reports whether a path is in the index.
func (idx *Index) IsTracked(path string) bool {
	return idx.Find(path) != nil
}

// GetAllPaths returns all paths in the index.
func (idx *Index) GetAllPaths() []string {
	paths := make([]string, len(idx.Entries))
	for i, e := range idx.Entries {
		paths[i] = e.Path
	}
	return paths
}

// ComputeHash returns the hash of the index content for comparison.
func (idx *Index) ComputeHash() hash.Hash {
	var buf bytes.Buffer
	for _, e := range idx.Entries {
		buf.Write([]byte(e.Path))
		buf.Write(e.Hash[:])
		fmt.Fprintf(&buf, "%d%d", e.Size, e.MtimeNs)
	}
	return hash.Compute(buf.Bytes())
}
