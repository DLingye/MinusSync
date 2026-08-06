package delta

import (
	"github.com/MinusSync/internal/hash"
)

// FastCDC implements content-defined chunking using the Gear hash algorithm.
// It produces variable-size chunks based on content, so that insertions/deletions
// in the middle of a file only affect local chunk boundaries.
type FastCDC struct {
	cfg ChunkConfig

	// Gear table for the rolling hash (256 random uint64 values)
	gearTable [256]uint64
}

// NewFastCDC creates a new FastCDC chunker.
func NewFastCDC(cfg ChunkConfig) *FastCDC {
	f := &FastCDC{cfg: cfg}
	f.initGearTable()
	return f
}

// initGearTable generates the gear hash lookup table using a deterministic PRNG.
func (f *FastCDC) initGearTable() {
	// Use a simple LCG to generate the gear table
	var state uint64 = 0x123456789ABCDEF0
	for i := 0; i < 256; i++ {
		state = state*6364136223846793005 + 1442695040888963407
		f.gearTable[i] = state
	}
}

// Split implements the Chunker interface.
func (f *FastCDC) Split(data []byte) ([]Chunk, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var chunks []Chunk
	offset := uint64(0)

	for offset < uint64(len(data)) {
		end := f.findChunkBoundary(data, offset)
		chunkData := data[offset:end]
		chunkHash := hash.Compute(chunkData)

		chunks = append(chunks, Chunk{
			Offset: offset,
			Length: uint32(len(chunkData)),
			Hash:   chunkHash,
			Data:   chunkData,
		})

		offset = end
	}

	return chunks, nil
}

// findChunkBoundary finds the next chunk boundary starting from offset.
func (f *FastCDC) findChunkBoundary(data []byte, offset uint64) uint64 {
	dataLen := uint64(len(data))

	minEnd := offset + uint64(f.cfg.MinSize)
	maxEnd := offset + uint64(f.cfg.MaxSize)

	if minEnd >= dataLen {
		return dataLen
	}
	if maxEnd > dataLen {
		maxEnd = dataLen
	}

	// Initialize rolling hash with the first 64 bytes of the window
	var rollingHash uint64
	windowStart := minEnd - 64
	if windowStart < offset {
		windowStart = offset
	}

	for i := windowStart; i < minEnd; i++ {
		rollingHash = (rollingHash << 1) + f.gearTable[data[i]]
	}

	// Normalized chunking with 3 levels
	masks := f.computeMasks()

	pos := minEnd
	for pos < maxEnd {
		// Slide the window
		rollingHash = (rollingHash << 1) + f.gearTable[data[pos]]

		// Check for chunk boundary at each mask level
		for _, mask := range masks {
			if rollingHash&mask == 0 {
				return pos + 1
			}
		}

		pos++
	}

	return maxEnd
}

// computeMasks returns the masks for normalized chunking levels.
func (f *FastCDC) computeMasks() []uint64 {
	// Mask values correspond to average chunk sizes:
	// 14 bits → ~16KB, 12 bits → ~4KB, 10 bits → ~1KB
	avgBits := uint64(0)
	for n := f.cfg.AvgSize; n > 1; n >>= 1 {
		avgBits++
	}

	return []uint64{
		(1 << (avgBits + 1)) - 1, // Large chunk mask
		(1 << (avgBits - 1)) - 1, // Medium chunk mask
		(1 << (avgBits - 3)) - 1, // Small chunk mask
	}
}

// BuildSyncPlan compares local chunks with remote chunk hashes.
func BuildSyncPlan(localChunks []Chunk, remoteHashes []hash.Hash) *SyncPlan {
	remoteSet := make(map[string]bool, len(remoteHashes))
	for _, h := range remoteHashes {
		remoteSet[h.Hex()] = true
	}

	plan := &SyncPlan{
		TotalChunks:  len(localChunks),
		ReusedChunks: 0,
	}

	for _, chunk := range localChunks {
		if remoteSet[chunk.Hash.Hex()] {
			plan.ReusedChunks++
		} else {
			plan.MissingChunks = append(plan.MissingChunks, chunk)
		}
	}

	return plan
}
