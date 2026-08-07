package object

import (
	"fmt"
	"strings"
	"time"

	"github.com/MinusSync/internal/hash"
)

// TagData holds the data for a tag object (annotated tag).
type TagData struct {
	Object    hash.Hash  // The object being tagged (usually a commit)
	Type      ObjectType // Type of the tagged object
	Name      string     // Tag name
	Tagger    string
	Email     string
	Timestamp int64 // Unix seconds
	Message   string
}

// NewTagData creates a TagData with the current time.
func NewTagData(object hash.Hash, objType ObjectType, name, tagger, email, message string) *TagData {
	return &TagData{
		Object:    object,
		Type:      objType,
		Name:      name,
		Tagger:    tagger,
		Email:     email,
		Timestamp: time.Now().Unix(),
		Message:   message,
	}
}

// SerializeTag converts tag data to the on-disk format.
func SerializeTag(t *TagData) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "object %s\n", t.Object.Hex())
	fmt.Fprintf(&b, "type %s\n", t.Type.String())
	fmt.Fprintf(&b, "tag %s\n", t.Name)
	fmt.Fprintf(&b, "tagger %s <%s> %d +0000\n", t.Tagger, t.Email, t.Timestamp)

	if t.Message != "" {
		fmt.Fprintf(&b, "\n%s\n", t.Message)
	}

	return []byte(b.String())
}

// ParseTag parses serialized tag data.
func ParseTag(data []byte) (*TagData, error) {
	lines := strings.Split(string(data), "\n")

	t := &TagData{}
	messageStart := -1

	for i, line := range lines {
		if line == "" {
			messageStart = i + 1
			break
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "object":
			h, err := hash.FromHex(value)
			if err != nil {
				return nil, fmt.Errorf("invalid object hash in tag: %w", err)
			}
			t.Object = h
		case "type":
			switch value {
			case "blob":
				t.Type = TypeBlob
			case "tree":
				t.Type = TypeTree
			case "commit":
				t.Type = TypeCommit
			case "tag":
				t.Type = TypeTag
			default:
				return nil, fmt.Errorf("unknown object type in tag: %q", value)
			}
		case "tag":
			t.Name = value
		case "tagger":
			t.Tagger, t.Email = parsePersonLine(value)
			// Parse timestamp if present
			if idx := strings.LastIndex(value, ">"); idx >= 0 {
				rest := strings.TrimSpace(value[idx+1:])
				if ts, err := GetTimestamp(rest); err == nil {
					t.Timestamp = ts
				}
			}
		}
	}

	if messageStart >= 0 && messageStart < len(lines) {
		t.Message = strings.Join(lines[messageStart:], "\n")
		t.Message = strings.TrimSpace(t.Message)
	}

	if t.Object.IsZero() {
		return nil, fmt.Errorf("tag missing object entry")
	}

	return t, nil
}

// ReadTag reads a tag object and returns its parsed data.
func ReadTag(objectsDir string, h hash.Hash) (*TagData, error) {
	content, header, err := ReadContent(objectsDir, h)
	if err != nil {
		return nil, err
	}
	if header.Type != TypeTag {
		return nil, &ErrUnexpectedType{Got: header.Type, Want: TypeTag}
	}
	return ParseTag(content)
}
