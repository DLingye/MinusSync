package protocol

import "github.com/MinusSync/internal/hash"

// EncodeUpdateRef encodes a ref update request.
func EncodeUpdateRef(name string, oldHash, newHash hash.Hash) []byte {
	buf := make([]byte, 1+len(name)+32+32)
	buf[0] = byte(len(name))
	copy(buf[1:], []byte(name))
	copy(buf[1+len(name):], oldHash[:])
	copy(buf[1+len(name)+32:], newHash[:])
	return buf
}

// DecodeUpdateRef decodes a ref update request.
func DecodeUpdateRef(payload []byte) (name string, oldHash, newHash hash.Hash, err error) {
	if len(payload) < 1 {
		return "", hash.Zero, hash.Zero, errInvalidPayload("UPDATE_REF")
	}
	nameLen := int(payload[0])
	if len(payload) < 1+nameLen+32+32 {
		return "", hash.Zero, hash.Zero, errInvalidPayload("UPDATE_REF")
	}
	name = string(payload[1 : 1+nameLen])
	copy(oldHash[:], payload[1+nameLen:1+nameLen+32])
	copy(newHash[:], payload[1+nameLen+32:1+nameLen+64])
	return name, oldHash, newHash, nil
}

// EncodeChunkQuery encodes a chunk query.
func EncodeChunkQuery(hashes []hash.Hash) []byte {
	buf := make([]byte, 4+len(hashes)*32)
	WriteUint32BE(buf[0:4], uint32(len(hashes)))
	for i, h := range hashes {
		copy(buf[4+i*32:], h[:])
	}
	return buf
}

// DecodeChunkQuery decodes a chunk query.
func DecodeChunkQuery(payload []byte) ([]hash.Hash, error) {
	if len(payload) < 4 {
		return nil, errInvalidPayload("CHUNK_QUERY")
	}
	count := int(ReadUint32BE(payload[0:4]))
	if len(payload) < 4+count*32 {
		return nil, errInvalidPayload("CHUNK_QUERY")
	}
	hashes := make([]hash.Hash, count)
	for i := 0; i < count; i++ {
		copy(hashes[i][:], payload[4+i*32:4+(i+1)*32])
	}
	return hashes, nil
}

// EncodeChunkHaves encodes chunk haves as a bitmap.
func EncodeChunkHaves(bitmap []bool) []byte {
	byteCount := (len(bitmap) + 7) / 8
	buf := make([]byte, 4+byteCount)
	WriteUint32BE(buf[0:4], uint32(len(bitmap)))
	for i, have := range bitmap {
		if have {
			byteIdx := i / 8
			bitIdx := i % 8
			buf[4+byteIdx] |= 1 << bitIdx
		}
	}
	return buf
}

// DecodeChunkHaves decodes a chunk haves bitmap.
func DecodeChunkHaves(payload []byte) ([]bool, error) {
	if len(payload) < 4 {
		return nil, errInvalidPayload("CHUNK_HAVES")
	}
	count := int(ReadUint32BE(payload[0:4]))
	byteCount := (count + 7) / 8
	if len(payload) < 4+byteCount {
		return nil, errInvalidPayload("CHUNK_HAVES")
	}
	bitmap := make([]bool, count)
	for i := 0; i < count; i++ {
		byteIdx := i / 8
		bitIdx := i % 8
		bitmap[i] = (payload[4+byteIdx] & (1 << bitIdx)) != 0
	}
	return bitmap, nil
}

// EncodeSearchQuery encodes a search query.
func EncodeSearchQuery(query, glob string, flags uint32) []byte {
	q := []byte(query)
	g := []byte(glob)
	buf := make([]byte, 4+2+len(q)+2+len(g))
	WriteUint32BE(buf[0:4], flags)
	buf[4] = byte(len(q) >> 8)
	buf[5] = byte(len(q))
	copy(buf[6:6+len(q)], q)
	pos := 6 + len(q)
	buf[pos] = byte(len(g) >> 8)
	buf[pos+1] = byte(len(g))
	copy(buf[pos+2:], g)
	return buf
}

// EncodeAuthRequest encodes an authentication request with supported methods.
func EncodeAuthRequest(methods []string) []byte {
	var buf []byte
	buf = append(buf, byte(len(methods)))
	for _, m := range methods {
		buf = append(buf, byte(len(m)))
		buf = append(buf, []byte(m)...)
	}
	return buf
}

// DecodeAuthResponse decodes an authentication response.
func DecodeAuthResponse(payload []byte) (method string, data []byte, err error) {
	if len(payload) < 2 {
		return "", nil, errInvalidPayload("AUTH_RESPONSE")
	}
	methodLen := int(payload[0])
	if len(payload) < 1+methodLen+4 {
		return "", nil, errInvalidPayload("AUTH_RESPONSE")
	}
	method = string(payload[1 : 1+methodLen])
	pos := 1 + methodLen
	dataLen := ReadUint32BE(payload[pos : pos+4])
	pos += 4
	if len(payload) < pos+int(dataLen) {
		return "", nil, errInvalidPayload("AUTH_RESPONSE")
	}
	data = payload[pos : pos+int(dataLen)]
	return method, data, nil
}
