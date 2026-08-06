package msyncd

import (
	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/repo"
)

// RepoManager manages a single repository for the server.
type RepoManager struct {
	repo     *repo.Repository
	readOnly bool
}

// NewRepoManager opens a repository for server use.
func NewRepoManager(path string, readOnly bool) (*RepoManager, error) {
	r, err := repo.Open(path)
	if err != nil {
		return nil, err
	}

	return &RepoManager{
		repo:     r,
		readOnly: readOnly,
	}, nil
}

// ListRefs returns all refs in the repository.
func (rm *RepoManager) ListRefs() (map[string]hash.Hash, error) {
	refs := make(map[string]hash.Hash)

	branches, err := rm.repo.Refs.ListBranches()
	if err != nil {
		return nil, err
	}
	for _, b := range branches {
		h, err := rm.repo.Refs.GetBranch(b)
		if err != nil {
			continue
		}
		refs["refs/heads/"+b] = h
	}

	tags, err := rm.repo.Refs.ListTags()
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		h, err := rm.repo.Refs.GetTag(t)
		if err != nil {
			continue
		}
		refs["refs/tags/"+t] = h
	}

	return refs, nil
}

// IsReadOnly reports whether this repository is read-only.
func (rm *RepoManager) IsReadOnly() bool {
	return rm.readOnly
}
