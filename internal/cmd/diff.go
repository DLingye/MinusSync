package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// shouldSkipPath reports whether a relative path should be excluded from scanning.
func shouldSkipPath(info os.FileInfo, relPath string) bool {
	if info.IsDir() && (relPath == ".msync" || strings.HasPrefix(relPath, ".msync/")) {
		return true
	}
	if !info.IsDir() && strings.HasPrefix(relPath, ".msync/") {
		return true
	}
	return false
}

func showDiff(r *repo.Repository, staged, statOnly bool) error {
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err != nil {
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
		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if shouldSkipPath(info, relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

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
	_, treeMap, err := object.ListTree(r.ObjectsPath(), treeHash)
	if err != nil {
		return err
	}

	for _, entry := range r.Index.Entries {
		if shouldSkipPath(nil, entry.Path) {
			continue
		}
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
		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if shouldSkipPath(info, relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		entry := r.Index.Find(relPath)
		if entry != nil {
			dirty, _ := r.Index.Dirty(relPath, info)
			if !dirty {
				if treeEntry, inTree := treeMap[relPath]; inTree {
					if treeEntry.Hash.Equal(entry.Hash) {
						return nil
					}
				}
			}
		}

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
	_ = treeMap
	return nil
}

// Ensure os import is used
var _ = os.Getpagesize
