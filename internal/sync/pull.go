package sync

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
)

// Pull fetches from a remote and updates the working tree.
func Pull(r *repo.Repository, remoteName string) error {
	if err := Fetch(r, remoteName); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	remoteHash, err := r.Refs.GetRemoteRef(remoteName, "HEAD")
	if err != nil {
		return fmt.Errorf("remote HEAD not found: %w", err)
	}

	localHash, _ := r.Refs.ResolveHEAD(r.HeadPath())

	if localHash.Equal(remoteHash) {
		fmt.Println("Already up to date.")
		return nil
	}

	fmt.Printf("Pull from %s: update to %s\n", remoteName, remoteHash.String())
	return nil
}
