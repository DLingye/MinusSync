package util

import (
	"bytes"
	"io"
	"os"

	"github.com/MinusSync/internal/hash"
)

// fingerprintSampleSize 是部分哈希指纹的采样点大小（字节）。
const fingerprintSampleSize = 4096

// ComputeFingerprint 计算文件内容的部分哈希指纹。
// 采用「头部 4KB + 中间 4KB + 尾部 4KB」三个采样点拼接后做 SHA-256，
// 用于快速判断文件是否被修改，无需读取完整文件。
// 对于小于 12KB 的文件，直接对完整内容做哈希（读取成本本来就低）。
func ComputeFingerprint(data []byte) hash.Hash {
	if len(data) <= fingerprintSampleSize*3 {
		return hash.Compute(data)
	}

	var buf bytes.Buffer
	buf.Write(data[:fingerprintSampleSize]) // 头部

	mid := len(data) / 2
	buf.Write(data[mid-fingerprintSampleSize/2 : mid+fingerprintSampleSize/2]) // 中间

	buf.Write(data[len(data)-fingerprintSampleSize:]) // 尾部

	return hash.Compute(buf.Bytes())
}

// ComputeFingerprintFromFile 从磁盘文件计算部分哈希指纹。
// 只读取头部、中间、尾部三个采样区，避免读完整文件。
// 对于小文件（<= 12KB）直接读取完整内容。
func ComputeFingerprintFromFile(path string) (hash.Hash, error) {
	info, err := os.Stat(path)
	if err != nil {
		return hash.Zero, err
	}

	size := info.Size()

	// 小文件直接全量读取
	if size <= fingerprintSampleSize*3 {
		data, err := os.ReadFile(path)
		if err != nil {
			return hash.Zero, err
		}
		return hash.Compute(data), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return hash.Zero, err
	}
	defer f.Close()

	var buf bytes.Buffer

	// 头部采样
	head := make([]byte, fingerprintSampleSize)
	if _, err := io.ReadFull(f, head); err != nil {
		return hash.Zero, err
	}
	buf.Write(head)

	// 中间采样
	mid := size / 2
	if _, err := f.Seek(mid-fingerprintSampleSize/2, io.SeekStart); err != nil {
		return hash.Zero, err
	}
	middle := make([]byte, fingerprintSampleSize)
	if _, err := io.ReadFull(f, middle); err != nil {
		return hash.Zero, err
	}
	buf.Write(middle)

	// 尾部采样
	if _, err := f.Seek(size-fingerprintSampleSize, io.SeekStart); err != nil {
		return hash.Zero, err
	}
	tail := make([]byte, fingerprintSampleSize)
	if _, err := io.ReadFull(f, tail); err != nil {
		return hash.Zero, err
	}
	buf.Write(tail)

	return hash.Compute(buf.Bytes()), nil
}
