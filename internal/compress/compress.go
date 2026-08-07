// Package compress provides zstd compression utilities for MinusSync.
// Uses github.com/klauspost/compress/zstd for pure-Go implementation.
package compress

import (
	"bytes"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	encoderPool sync.Pool
	decoderPool sync.Pool
)

func init() {
	// Initialize with default encoder/decoder for the pool
	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	encoderPool.New = func() interface{} {
		e, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
		return e
	}
	encoderPool.Put(enc)

	dec, _ := zstd.NewReader(nil)
	decoderPool.New = func() interface{} {
		d, _ := zstd.NewReader(nil)
		return d
	}
	decoderPool.Put(dec)
}

// GetEncoder returns a pooled zstd encoder.
func GetEncoder() *zstd.Encoder {
	return encoderPool.Get().(*zstd.Encoder)
}

// PutEncoder returns a zstd encoder to the pool.
func PutEncoder(enc *zstd.Encoder) {
	encoderPool.Put(enc)
}

// GetDecoder returns a pooled zstd decoder.
func GetDecoder() *zstd.Decoder {
	return decoderPool.Get().(*zstd.Decoder)
}

// PutDecoder returns a zstd decoder to the pool.
func PutDecoder(dec *zstd.Decoder) {
	decoderPool.Put(dec)
}

// Compress compresses data using zstd.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	enc := GetEncoder()
	defer PutEncoder(enc)

	enc.Reset(&buf)
	if _, err := enc.Write(data); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress decompresses zstd-compressed data.
func Decompress(data []byte) ([]byte, error) {
	dec := GetDecoder()
	defer PutDecoder(dec)

	dec.Reset(bytes.NewReader(data))
	return io.ReadAll(dec)
}

// CompressTo compresses data and writes to w.
func CompressTo(w io.Writer, data []byte) error {
	enc := GetEncoder()
	defer PutEncoder(enc)

	enc.Reset(w)
	if _, err := enc.Write(data); err != nil {
		return err
	}
	return enc.Close()
}

// DecompressFrom reads zstd-compressed data from r and decompresses.
func DecompressFrom(r io.Reader) ([]byte, error) {
	dec := GetDecoder()
	defer PutDecoder(dec)

	dec.Reset(r)
	return io.ReadAll(dec)
}
