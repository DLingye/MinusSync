package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func diffCmd() *cobra.Command {
	var staged bool
	var statOnly bool

	cmd := &cobra.Command{
		Use:   "diff [--staged] [ref] [--] [path]",
		Short: "Show changes",
		Long:  "Show changes between commits, the index, and the working tree.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			return showDiff(r, staged, statOnly)
		},
	}

	cmd.Flags().BoolVar(&staged, "staged", false, "Show staged changes")
	cmd.Flags().BoolVar(&statOnly, "stat", false, "Show statistics only")
	return cmd
}

func showDiff(r *repo.Repository, staged, statOnly bool) error {
	// Get HEAD tree
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err != nil {
		// No HEAD commit yet — everything is new
		return showDiffWorkingVsIndex(r, statOnly)
	}

	headCommit, err := object.ReadCommit(r.ObjectsPath(), headHash)
	if err != nil {
		return showDiffWorkingVsIndex(r, statOnly)
	}

	if staged {
		return showDiffIndexVsTree(r, headCommit.Tree, statOnly)
	}
	return showDiffWorkingVsTree(r, headCommit.Tree, statOnly)
}

func showDiffWorkingVsIndex(r *repo.Repository, statOnly bool) error {
	cwd, _ := util.CurrentDir()
	files := 0

	util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		if info.IsDir() {
			if filepath.Base(path) == ".msync" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		entry := r.Index.Find(relPath)
		if entry == nil {
			if statOnly {
				files++
			} else {
				fmt.Printf("new file: %s\n", relPath)
			}
			return nil
		}

		dirty, _ := r.Index.Dirty(relPath, info)
		if dirty {
			if statOnly {
				files++
			} else {
				fmt.Printf("modified: %s\n", relPath)
			}
		}
		return nil
	})

	if statOnly {
		fmt.Printf("%d files changed\n", files)
	}
	return nil
}

func showDiffIndexVsTree(r *repo.Repository, treeHash hash.Hash, statOnly bool) error {
	// Get files in tree
	_, treeMap, err := object.ListTree(r.ObjectsPath(), treeHash)
	if err != nil {
		return err
	}

	for _, entry := range r.Index.Entries {
		treeEntry, inTree := treeMap[entry.Path]
		if !inTree {
			fmt.Printf("new file:   %s\n", entry.Path)
		} else if !treeEntry.Hash.Equal(entry.Hash) {
			fmt.Printf("modified:   %s\n", entry.Path)
		}
	}

	return nil
}

func showDiffWorkingVsTree(r *repo.Repository, treeHash hash.Hash, statOnly bool) error {
	cwd, _ := util.CurrentDir()
	_, treeMap, err := object.ListTree(r.ObjectsPath(), treeHash)
	if err != nil {
		treeMap = make(map[string]object.TreeEntry)
	}

	files := 0
	util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		if info.IsDir() {
			if filepath.Base(path) == ".msync" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		// Check index entry first (fast path)
		entry := r.Index.Find(relPath)

		if entry != nil {
			dirty, _ := r.Index.Dirty(relPath, info)
			if !dirty {
				// File matches index — check if index differs from tree
				if treeEntry, inTree := treeMap[relPath]; inTree {
					if treeEntry.Hash.Equal(entry.Hash) {
						return nil // No change
					}
				}
			}
		}

		// File is modified or new
		if statOnly {
			files++
		} else {
			_, inTree := treeMap[relPath]
			if inTree {
				fmt.Printf("modified:   %s\n", relPath)
			} else {
				fmt.Printf("new file:   %s\n", relPath)
			}
		}
		return nil
	})

	if statOnly {
		fmt.Printf("%d files changed\n", files)
	}
	return nil
}

// Ensure os import is used
var _ = os.Getpagesize
