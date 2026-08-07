package object

import (
	"os"

	"github.com/MinusSync/internal/hash"
)

// CreateBlob reads a file from disk and creates a blob object from its content.
// Returns the SHA-256 hash of the blob object.
func CreateBlob(objectsDir, filePath string) (hash.Hash, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return hash.Zero, err
	}
	return WriteBlob(objectsDir, content)
}

// ReadBlob reads a blob object and returns its content.
func ReadBlob(objectsDir string, h hash.Hash) ([]byte, error) {
	content, header, err := ReadContent(objectsDir, h)
	if err != nil {
		return nil, err
	}
	if header.Type != TypeBlob {
		return nil, &ErrUnexpectedType{Got: header.Type, Want: TypeBlob}
	}
	return content, nil
}

// ErrUnexpectedType is returned when an object has an unexpected type.
type ErrUnexpectedType struct {
	Got  ObjectType
	Want ObjectType
}

func (e *ErrUnexpectedType) Error() string {
	return "expected object type " + e.Want.String() + ", got " + e.Got.String()
}
