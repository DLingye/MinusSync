package sync

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
)

// Pull fetches from a remote and merges into the current branch.
func Pull(r *repo.Repository, remoteName, branch string) error {
	// Fetch first
	if err := Fetch(r, remoteName); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	// Merge the remote tracking branch
	remoteBranch := fmt.Sprintf("refs/heads/%s", branch)
	remoteHash, err := r.Refs.GetRemoteRef(remoteName, remoteBranch)
	if err != nil {
		return fmt.Errorf("remote tracking ref not found: %w", err)
	}

	_ = remoteHash

	fmt.Printf("Pull from %s/%s: merge not yet fully implemented\n", remoteName, branch)
	return nil
}
