// Package protocol defines the MinusSync wire protocol.
package protocol

// Handshake magic bytes.
var HandshakeMagic = [4]byte{'M', 'S', 'Y', 'N'}

// Current protocol version.
const (
	VersionMajor = 1
	VersionMinor = 0
)

// Message type constants.
const (
	MsgListRefs     uint16 = 0x0001
	MsgRefLine      uint16 = 0x0002
	MsgFetchRequest uint16 = 0x0003
	MsgWanted       uint16 = 0x0004
	MsgHave         uint16 = 0x0005
	MsgFetchDone    uint16 = 0x0006
	MsgPackHeader   uint16 = 0x0007
	MsgPackObject   uint16 = 0x0008
	MsgPushRequest  uint16 = 0x0009
	MsgUpdateRef    uint16 = 0x000A
	MsgWantPack     uint16 = 0x000B
	MsgOK           uint16 = 0x000C
	MsgError        uint16 = 0x000D
	MsgAuthRequest  uint16 = 0x000E
	MsgAuthResponse uint16 = 0x000F
	MsgChunkQuery   uint16 = 0x0010
	MsgChunkHaves   uint16 = 0x0011
	MsgSearchReq    uint16 = 0x0020
	MsgSearchResult uint16 = 0x0021
	MsgStreamEnd    uint16 = 0xFFFF
)

// Error codes.
const (
	ErrRepoNotFound    uint32 = 0x0001
	ErrAuthRequired    uint32 = 0x0002
	ErrAuthFailed      uint32 = 0x0003
	ErrRefNotFound     uint32 = 0x0004
	ErrNonFastForward  uint32 = 0x0005
	ErrObjectMissing   uint32 = 0x0006
	ErrInternal        uint32 = 0x0007
	ErrBadRequest      uint32 = 0x0008
	ErrPermissionDenied uint32 = 0x0009
)

// ErrorCodeName returns a human-readable name for an error code.
func ErrorCodeName(code uint32) string {
	switch code {
	case ErrRepoNotFound:
		return "repo not found"
	case ErrAuthRequired:
		return "auth required"
	case ErrAuthFailed:
		return "auth failed"
	case ErrRefNotFound:
		return "ref not found"
	case ErrNonFastForward:
		return "non-fast-forward"
	case ErrObjectMissing:
		return "object missing"
	case ErrInternal:
		return "internal error"
	case ErrBadRequest:
		return "bad request"
	case ErrPermissionDenied:
		return "permission denied"
	default:
		return "unknown error"
	}
}

// MessageName returns a human-readable name for a message type.
func MessageName(msgType uint16) string {
	switch msgType {
	case MsgListRefs:
		return "LIST_REFS"
	case MsgRefLine:
		return "REF_LINE"
	case MsgFetchRequest:
		return "FETCH_REQUEST"
	case MsgWanted:
		return "WANTED"
	case MsgHave:
		return "HAVE"
	case MsgFetchDone:
		return "FETCH_DONE"
	case MsgPackHeader:
		return "PACK_HEADER"
	case MsgPackObject:
		return "PACK_OBJECT"
	case MsgPushRequest:
		return "PUSH_REQUEST"
	case MsgUpdateRef:
		return "UPDATE_REF"
	case MsgWantPack:
		return "WANT_PACK"
	case MsgOK:
		return "OK"
	case MsgError:
		return "ERROR"
	case MsgAuthRequest:
		return "AUTH_REQUEST"
	case MsgAuthResponse:
		return "AUTH_RESPONSE"
	case MsgChunkQuery:
		return "CHUNK_QUERY"
	case MsgChunkHaves:
		return "CHUNK_HAVES"
	case MsgSearchReq:
		return "SEARCH_REQ"
	case MsgSearchResult:
		return "SEARCH_RESULT"
	case MsgStreamEnd:
		return "STREAM_END"
	default:
		return "UNKNOWN"
	}
}
