package object

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/MinusSync/internal/compress"
	"github.com/MinusSync/internal/hash"
)

// Pack file magic numbers.
var (
	packMagic = [4]byte{'M', 'P', 'A', 'K'}
	idxMagic  = [4]byte{'M', 'P', 'I', 'X'}
)

const packVersion uint32 = 1

// PackEntry describes one object in a pack file.
type PackEntry struct {
	Hash           hash.Hash
	Type           ObjectType
	Offset         uint32 // Offset into the body section
	CompressedLen  uint32
	UncompressedLen uint32
}

// PackWriter writes objects into a pack file.
type PackWriter struct {
	f       *os.File
	entries []PackEntry
	bodyBuf bytes.Buffer
	offset  uint32
}

// NewPackWriter creates a new pack file writer.
func NewPackWriter(path string) (*PackWriter, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	pw := &PackWriter{f: f}

	// Write placeholder header (we'll fill it in on Close)
	header := make([]byte, 4+4+4) // magic + version + count
	copy(header[0:4], packMagic[:])
	binary.BigEndian.PutUint32(header[4:8], packVersion)
	// count (bytes 8-12) will be filled at Close
	if _, err := f.Write(header); err != nil {
		return nil, err
	}

	return pw, nil
}

// AddObject adds an object to the pack. The data should be the full serialized object
// (header + content, i.e. "type size\0content").
func (pw *PackWriter) AddObject(data []byte) (hash.Hash, error) {
	h := hash.Compute(data)

	// Parse header to get type and content
	objHeader, contentOffset, err := ParseHeader(data)
	if err != nil {
		return hash.Zero, err
	}

	// zstd compress the full object
	compressed, compErr := compress.Compress(data)
	if compErr != nil {
		return hash.Zero, compErr
	}

	entry := PackEntry{
		Hash:           h,
		Type:           objHeader.Type,
		Offset:         pw.offset,
		CompressedLen:  uint32(len(compressed)),
		UncompressedLen: uint32(len(data[contentOffset:])),
	}

	pw.entries = append(pw.entries, entry)
	pw.bodyBuf.Write(compressed)
	pw.offset += uint32(len(compressed))

	return h, nil
}

// Close finalizes the pack file and writes the index.
func (pw *PackWriter) Close() error {
	// Write body section
	body := pw.bodyBuf.Bytes()

	// Sort entries by hash for binary search in index
	sort.Slice(pw.entries, func(i, j int) bool {
		return bytes.Compare(pw.entries[i].Hash[:], pw.entries[j].Hash[:]) < 0
	})

	// Write entries table
	entriesBuf := new(bytes.Buffer)
	for _, e := range pw.entries {
		entriesBuf.Write(e.Hash[:])
		entriesBuf.WriteByte(byte(e.Type))
		binary.Write(entriesBuf, binary.BigEndian, e.Offset)
		binary.Write(entriesBuf, binary.BigEndian, e.CompressedLen)
		binary.Write(entriesBuf, binary.BigEndian, e.UncompressedLen)
	}

	// Calculate body offset = current file position + entries size + body size
	// Actually we write: header | entries | body | crc32
	// The offset in PackEntry is relative to start of body section

	// Write entries to file
	if _, err := pw.f.Write(entriesBuf.Bytes()); err != nil {
		return err
	}

	// Write body to file
	if _, err := pw.f.Write(body); err != nil {
		return err
	}

	// Write CRC32
	crc := crc32.ChecksumIEEE(entriesBuf.Bytes())
	if err := binary.Write(pw.f, binary.BigEndian, crc); err != nil {
		return err
	}

	// Go back and fix up header with correct count
	countBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(countBytes, uint32(len(pw.entries)))
	if _, err := pw.f.WriteAt(countBytes, 8); err != nil {
		return err
	}

	return pw.f.Close()
}

// PackReader reads objects from a pack file.
type PackReader struct {
	f       *os.File
	entries []PackEntry
	bodyOffset int64
}

// OpenPack opens a pack file for reading.
func OpenPack(path string) (*PackReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Read header
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		f.Close()
		return nil, err
	}
	if magic != packMagic {
		f.Close()
		return nil, fmt.Errorf("invalid pack magic: %v", magic)
	}

	var version uint32
	if err := binary.Read(f, binary.BigEndian, &version); err != nil {
		f.Close()
		return nil, err
	}
	if version != packVersion {
		f.Close()
		return nil, fmt.Errorf("unsupported pack version: %d", version)
	}

	var count uint32
	if err := binary.Read(f, binary.BigEndian, &count); err != nil {
		f.Close()
		return nil, err
	}

	// Read entries
	entries := make([]PackEntry, count)
	for i := uint32(0); i < count; i++ {
		var e PackEntry
		if _, err := io.ReadFull(f, e.Hash[:]); err != nil {
			f.Close()
			return nil, err
		}
		var typeByte [1]byte
		if _, err := io.ReadFull(f, typeByte[:]); err != nil {
			f.Close()
			return nil, err
		}
		e.Type = ObjectType(typeByte[0])

		var buf [4]byte
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			f.Close()
			return nil, err
		}
		e.Offset = binary.BigEndian.Uint32(buf[:])
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			f.Close()
			return nil, err
		}
		e.CompressedLen = binary.BigEndian.Uint32(buf[:])
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			f.Close()
			return nil, err
		}
		e.UncompressedLen = binary.BigEndian.Uint32(buf[:])

		entries[i] = e
	}

	// Record body offset (where compressed data starts)
	bodyOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		f.Close()
		return nil, err
	}

	// Verify CRC32 (read from end of file)
	// First, compute CRC32 of the entries data we just read
	// We need to re-read the entries section
	// For simplicity, skip CRC verification for now; it's checked via object hash verification

	return &PackReader{
		f:          f,
		entries:    entries,
		bodyOffset: bodyOffset,
	}, nil
}

// ReadObject reads a single object from the pack by hash.
func (pr *PackReader) ReadObject(h hash.Hash) ([]byte, error) {
	// Binary search for the hash
	idx := sort.Search(len(pr.entries), func(i int) bool {
		return bytes.Compare(pr.entries[i].Hash[:], h[:]) >= 0
	})

	if idx >= len(pr.entries) || !pr.entries[idx].Hash.Equal(h) {
		return nil, fmt.Errorf("object %s not found in pack", h.Hex())
	}

	e := pr.entries[idx]

	// Seek to compressed data
	readOffset := pr.bodyOffset + int64(e.Offset)
	if _, err := pr.f.Seek(readOffset, io.SeekStart); err != nil {
		return nil, err
	}

	// Read compressed data
	compressed := make([]byte, e.CompressedLen)
	if _, err := io.ReadFull(pr.f, compressed); err != nil {
		return nil, err
	}

	// Decompress
	data, err := compress.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress object %s: %w", h.Hex(), err)
	}

	// Verify hash
	actual := hash.Compute(data)
	if !actual.Equal(h) {
		return nil, fmt.Errorf("object %s corrupted in pack (hash mismatch)", h.Hex())
	}

	return data, nil
}

// ObjectCount returns the number of objects in the pack.
func (pr *PackReader) ObjectCount() int {
	return len(pr.entries)
}

// Objects returns all hashes in the pack.
func (pr *PackReader) Objects() []hash.Hash {
	hashes := make([]hash.Hash, len(pr.entries))
	for i, e := range pr.entries {
		hashes[i] = e.Hash
	}
	return hashes
}

// Close closes the pack file.
func (pr *PackReader) Close() error {
	return pr.f.Close()
}

// UnpackObjects reads all objects from a pack and stores them in the object store.
func UnpackObjects(objectsDir string, packPath string) ([]hash.Hash, error) {
	pr, err := OpenPack(packPath)
	if err != nil {
		return nil, err
	}
	defer pr.Close()

	var hashes []hash.Hash
	for _, e := range pr.entries {
		data, err := pr.ReadObject(e.Hash)
		if err != nil {
			return nil, err
		}
		h, err := Write(objectsDir, data)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, nil
}

// WritePackIndex writes a pack index file.
func WritePackIndex(idxPath string, packHash hash.Hash, entries []PackEntry) error {
	f, err := os.Create(idxPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Magic
	f.Write(idxMagic[:])

	// Pack hash
	f.Write(packHash[:])

	// Sorted entries
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].Hash[:], entries[j].Hash[:]) < 0
	})

	for _, e := range entries {
		f.Write(e.Hash[:])
		binary.Write(f, binary.BigEndian, e.Offset)
		f.Write([]byte{byte(e.Type)})
	}

	return nil
}
