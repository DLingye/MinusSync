// Package gc provides garbage collection for unreachable objects.
package gc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/refs"
)

// Report holds the result of a GC run.
type Report struct {
	TotalObjects       int
	ReachableObjects   int
	UnreachableObjects int
	RecoveredBytes     int64
	Pruned             bool
}

// Collect performs garbage collection on the repository.
func Collect(repoPath, objectsDir, refsDir, headPath string, prune bool, verbose bool) (*Report, error) {
	report := &Report{}

	// 1. Mark: find all reachable objects from refs
	reachable := make(map[string]bool)

	// Start from all branches
	refMgr := refs.New(refsDir)
	branches, _ := refMgr.ListBranches()
	for _, b := range branches {
		h, err := refMgr.GetBranch(b)
		if err != nil {
			continue
		}
		markReachable(objectsDir, h, reachable)
	}

	// Start from all tags
	tags, _ := refMgr.ListTags()
	for _, t := range tags {
		h, err := refMgr.GetTag(t)
		if err != nil {
			continue
		}
		markReachable(objectsDir, h, reachable)
	}

	// Start from HEAD
	headHash, err := refMgr.ResolveHEAD(headPath)
	if err == nil && !headHash.IsZero() {
		markReachable(objectsDir, headHash, reachable)
	}

	// Also mark remote tracking refs
	remotes, _ := refMgr.ListRemotes()
	for _, remote := range remotes {
		remoteBranches, _ := refMgr.ListRemoteBranches(remote)
		for _, b := range remoteBranches {
			h, err := refMgr.GetRemoteRef(remote, b)
			if err != nil {
				continue
			}
			markReachable(objectsDir, h, reachable)
		}
	}

	report.ReachableObjects = len(reachable)

	// 2. Sweep: enumerate all objects and remove unreachable ones
	prefixDir := filepath.Join(objectsDir)
	entries, err := os.ReadDir(prefixDir)
	if err != nil {
		return report, err
	}

	for _, prefix := range entries {
		if !prefix.IsDir() || len(prefix.Name()) != 2 {
			continue
		}

		objDir := filepath.Join(objectsDir, prefix.Name())
		objEntries, err := os.ReadDir(objDir)
		if err != nil {
			continue
		}

		for _, objFile := range objEntries {
			if objFile.IsDir() {
				continue
			}

			report.TotalObjects++
			fullHash := prefix.Name() + objFile.Name()

			if !reachable[fullHash] {
				report.UnreachableObjects++
				if verbose {
					fmt.Printf("unreachable: %s\n", fullHash[:7])
				}

				if prune {
					objPath := filepath.Join(objDir, objFile.Name())
					info, _ := objFile.Info()
					if info != nil {
						report.RecoveredBytes += info.Size()
					}
					if err := os.Remove(objPath); err != nil && verbose {
						fmt.Printf("  failed to remove: %v\n", err)
					} else {
						report.Pruned = true
					}
				}
			}
		}

		// Remove empty prefix directories
		if prune {
			remaining, _ := os.ReadDir(objDir)
			if len(remaining) == 0 {
				os.Remove(objDir)
			}
		}
	}

	return report, nil
}

// markReachable marks an object and all objects it references as reachable.
func markReachable(objectsDir string, h hash.Hash, reachable map[string]bool) {
	hex := h.Hex()
	if reachable[hex] {
		return
	}
	reachable[hex] = true

	// Read the object to find its references
	header, err := object.ReadHeader(objectsDir, h)
	if err != nil {
		return
	}

	switch header.Type {
	case object.TypeCommit:
		commit, err := object.ReadCommit(objectsDir, h)
		if err != nil {
			return
		}
		markReachable(objectsDir, commit.Tree, reachable)
		for _, p := range commit.Parents {
			markReachable(objectsDir, p, reachable)
		}

	case object.TypeTree:
		entries, err := object.ReadTree(objectsDir, h)
		if err != nil {
			return
		}
		for _, e := range entries {
			markReachable(objectsDir, e.Hash, reachable)
		}

	case object.TypeTag:
		tag, err := object.ReadTag(objectsDir, h)
		if err != nil {
			return
		}
		markReachable(objectsDir, tag.Object, reachable)

	case object.TypeBlob:
		// Blobs have no references
	}
}

// Ensure fmt is used
var _ = fmt.Sprintf
