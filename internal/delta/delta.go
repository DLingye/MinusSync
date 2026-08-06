package delta

import (
	"encoding/binary"
	"fmt"
)

// OpType for delta operations.
const (
	OpCopy   uint8 = 0
	OpInsert uint8 = 1
)

// DeltaOp is a single operation in a delta.
type DeltaOp struct {
	OpType    uint8  // OpCopy or OpInsert
	SrcOffset uint64 // For OpCopy: offset in base
	DstOffset uint64 // For OpCopy/OpInsert: offset in target
	Length    uint32
	Data      []byte // For OpInsert: the raw data to insert
}

// ComputeDelta computes a delta between base and target.
// Uses a rolling hash to find matching sub-sequences.
func ComputeDelta(base, target []byte) ([]DeltaOp, error) {
	if len(base) == 0 {
		return []DeltaOp{{OpType: OpInsert, Length: uint32(len(target)), Data: target}}, nil
	}

	var ops []DeltaOp
	tPos := uint64(0)

	// Build a hash map of base chunks for fast lookup
	chunkSize := 16
	baseIndex := buildHashIndex(base, chunkSize)

	for tPos < uint64(len(target)) {
		// Try to find a match in base
		bestMatch := findBestMatch(base, target[tPos:], baseIndex, chunkSize)

		if bestMatch.length >= chunkSize {
			// Add any preceding insert
			if tPos > 0 && len(ops) > 0 && ops[len(ops)-1].OpType == OpInsert {
				// Extend previous insert
			}

			if bestMatch.length > 0 {
				ops = append(ops, DeltaOp{
					OpType:    OpCopy,
					SrcOffset: bestMatch.offset,
					Length:    uint32(bestMatch.length),
				})
				tPos += uint64(bestMatch.length)
			}
		} else {
			// No match found — insert one byte
			ops = append(ops, DeltaOp{
				OpType: OpInsert,
				Length: 1,
				Data:   target[tPos : tPos+1],
			})
			tPos++
		}
	}

	return compactOps(ops), nil
}

// ApplyDelta applies delta operations to base to produce target.
func ApplyDelta(base []byte, ops []DeltaOp) ([]byte, error) {
	var result []byte

	for _, op := range ops {
		switch op.OpType {
		case OpCopy:
			srcEnd := op.SrcOffset + uint64(op.Length)
			if srcEnd > uint64(len(base)) {
				return nil, fmt.Errorf("copy exceeds base: offset=%d len=%d base_len=%d",
					op.SrcOffset, op.Length, len(base))
			}
			result = append(result, base[op.SrcOffset:srcEnd]...)
		case OpInsert:
			result = append(result, op.Data...)
		}
	}

	return result, nil
}

type match struct {
	offset uint64
	length int
}

func buildHashIndex(data []byte, chunkSize int) map[uint32][]uint64 {
	index := make(map[uint32][]uint64)
	if len(data) < chunkSize {
		return index
	}

	for i := 0; i <= len(data)-chunkSize; i++ {
		h := rollingHash32(data[i : i+chunkSize])
		index[h] = append(index[h], uint64(i))
	}
	return index
}

func findBestMatch(base, target []byte, index map[uint32][]uint64, chunkSize int) match {
	if len(target) < chunkSize {
		return match{}
	}

	h := rollingHash32(target[:chunkSize])
	offsets, ok := index[h]
	if !ok {
		return match{}
	}

	best := match{}
	for _, off := range offsets {
		// Extend the match as far as possible
		length := chunkSize
		for off+uint64(length) < uint64(len(base)) &&
			length < len(target) &&
			base[off+uint64(length)] == target[length] {
			length++
		}
		if length > best.length {
			best = match{offset: off, length: length}
		}
	}

	return best
}

func rollingHash32(data []byte) uint32 {
	var h uint32
	for _, b := range data {
		h = h*31 + uint32(b)
	}
	return h
}

func compactOps(ops []DeltaOp) []DeltaOp {
	if len(ops) <= 1 {
		return ops
	}

	var result []DeltaOp
	for _, op := range ops {
		if op.Length == 0 {
			continue
		}
		if len(result) > 0 &&
			result[len(result)-1].OpType == OpInsert &&
			op.OpType == OpInsert {
			// Merge consecutive inserts
			last := &result[len(result)-1]
			last.Data = append(last.Data, op.Data...)
			last.Length += op.Length
		} else {
			result = append(result, op)
		}
	}
	return result
}

// EncodeDelta serializes delta operations for transmission.
func EncodeDelta(ops []DeltaOp) []byte {
	var buf []byte
	for _, op := range ops {
		buf = append(buf, op.OpType)
		switch op.OpType {
		case OpCopy:
			tmp := make([]byte, 12)
			binary.BigEndian.PutUint64(tmp[0:8], op.SrcOffset)
			binary.BigEndian.PutUint32(tmp[8:12], op.Length)
			buf = append(buf, tmp...)
		case OpInsert:
			tmp := make([]byte, 4)
			binary.BigEndian.PutUint32(tmp, uint32(len(op.Data)))
			buf = append(buf, tmp...)
			buf = append(buf, op.Data...)
		}
	}
	return buf
}

// DecodeDelta deserializes delta operations.
func DecodeDelta(data []byte) ([]DeltaOp, error) {
	var ops []DeltaOp
	pos := 0

	for pos < len(data) {
		if pos+1 > len(data) {
			break
		}
		opType := data[pos]
		pos++

		switch opType {
		case OpCopy:
			if pos+12 > len(data) {
				return nil, fmt.Errorf("truncated copy op")
			}
			op := DeltaOp{
				OpType:    OpCopy,
				SrcOffset: binary.BigEndian.Uint64(data[pos : pos+8]),
				Length:    binary.BigEndian.Uint32(data[pos+8 : pos+12]),
			}
			pos += 12
			ops = append(ops, op)
		case OpInsert:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("truncated insert op")
			}
			dataLen := binary.BigEndian.Uint32(data[pos : pos+4])
			pos += 4
			if pos+int(dataLen) > len(data) {
				return nil, fmt.Errorf("truncated insert data")
			}
			op := DeltaOp{
				OpType: OpInsert,
				Length: dataLen,
				Data:   data[pos : pos+int(dataLen)],
			}
			pos += int(dataLen)
			ops = append(ops, op)
		default:
			return nil, fmt.Errorf("unknown delta op type: %d", opType)
		}
	}

	return ops, nil
}

// Ensure fmt is imported
var _ = fmt.Sprintf
