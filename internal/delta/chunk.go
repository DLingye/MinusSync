// Package delta provides binary delta computation and chunking for efficient sync.
package delta

import (
	"github.com/MinusSync/internal/hash"
)

// ChunkConfig holds FastCDC parameters.
type ChunkConfig struct {
	MinSize int // Minimum chunk size (default: 2048)
	AvgSize int // Average chunk size (default: 8192)
	MaxSize int // Maximum chunk size (default: 32768)
}

// DefaultChunkConfig returns sensible defaults.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		MinSize: 2048,
		AvgSize: 8192,
		MaxSize: 32768,
	}
}

// Chunk represents one content-defined segment of a file.
type Chunk struct {
	Offset uint64    // Byte offset in the source file
	Length uint32    // Length of chunk content
	Hash   hash.Hash // SHA-256 of chunk content
	Data   []byte    // Actual chunk bytes
}

// Chunker is the interface for content-defined chunking.
type Chunker interface {
	// Split splits a byte slice into content-defined chunks.
	Split(data []byte) ([]Chunk, error)
}

// SyncPlan describes what to transfer for efficient binary sync.
type SyncPlan struct {
	MissingChunks []Chunk   // Chunks the remote doesn't have
	TotalChunks   int       // Total number of chunks
	ReusedChunks  int       // Number of chunks already on remote
}
