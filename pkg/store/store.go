// Package store provides a high-level object store interface for MinusSync.
package store

import (
	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
)

// Store is a high-level wrapper around the object store.
type Store struct {
	objectsDir string
}

// New creates a new Store.
func New(objectsDir string) *Store {
	return &Store{objectsDir: objectsDir}
}

// Put stores data as a blob and returns its hash.
func (s *Store) Put(data []byte) (hash.Hash, error) {
	return object.WriteBlob(s.objectsDir, data)
}

// Get retrieves blob content by hash.
func (s *Store) Get(h hash.Hash) ([]byte, error) {
	return object.ReadBlob(s.objectsDir, h)
}

// Exists checks if an object exists.
func (s *Store) Exists(h hash.Hash) bool {
	return object.Exists(s.objectsDir, h)
}

// PutCommit stores a commit and returns its hash.
func (s *Store) PutCommit(c *object.CommitData) (hash.Hash, error) {
	return object.WriteCommit(s.objectsDir, c)
}

// GetCommit retrieves a commit by hash.
func (s *Store) GetCommit(h hash.Hash) (*object.CommitData, error) {
	return object.ReadCommit(s.objectsDir, h)
}

// PutTree stores a tree and returns its hash.
func (s *Store) PutTree(entries []object.TreeEntry) (hash.Hash, error) {
	return object.WriteTree(s.objectsDir, entries)
}

// GetTree retrieves a tree by hash.
func (s *Store) GetTree(h hash.Hash) ([]object.TreeEntry, error) {
	return object.ReadTree(s.objectsDir, h)
}
