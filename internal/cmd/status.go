package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		Long:  "Show the status of the working tree compared to the index and HEAD.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			return showStatus(r, short)
		},
	}

	cmd.Flags().BoolVarP(&short, "short", "s", false, "Short format output")
	return cmd
}

func showStatus(r *repo.Repository, short bool) error {
	// Get current branch
	branch, err := r.Refs.CurrentBranch(r.HeadPath())
	if err != nil {
		return err
	}

	if !short {
		if branch != "" {
			fmt.Printf("On branch %s\n", branch)
		} else {
			headHash, _ := r.Refs.ResolveHEAD(r.HeadPath())
			fmt.Printf("HEAD detached at %s\n", headHash.String())
		}
	}

	// Get the HEAD tree for comparison
	var headTreeEntries map[string]object.TreeEntry
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err == nil && !headHash.IsZero() {
		headCommit, err := object.ReadCommit(r.ObjectsPath(), headHash)
		if err == nil {
			entries, err := object.ReadTree(r.ObjectsPath(), headCommit.Tree)
			if err == nil {
				headTreeEntries = make(map[string]object.TreeEntry)
				for _, e := range entries {
					headTreeEntries[e.Name] = e
				}
			}
		}
	}

	// Walk working directory
	var staged, modified, untracked []string

	cwd, _ := util.CurrentDir()
	util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		if info.IsDir() {
			name := filepath.Base(path)
			if name == ".msync" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		// Check index
		entry := r.Index.Find(relPath)
		if entry != nil {
			// In index — check if modified
			dirty, _ := r.Index.Dirty(relPath, info)
			if dirty {
				modified = append(modified, relPath)
			}
		} else {
			// Not in index — check if in HEAD tree
			if _, inHead := headTreeEntries[relPath]; inHead {
				modified = append(modified, relPath)
			} else {
				untracked = append(untracked, relPath)
			}
		}

		return nil
	})

	// Check for deleted files (in index but not on disk)
	for _, entry := range r.Index.Entries {
		absPath := filepath.Join(r.Path, entry.Path)
		if !util.FileExists(absPath) {
			modified = append(modified, entry.Path+" (deleted)")
		}
	}

	// Print results
	if len(staged) > 0 {
		fmt.Println("\nChanges to be committed:")
		for _, f := range staged {
			fmt.Printf("  new file:   %s\n", f)
		}
	}

	if len(modified) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		for _, f := range modified {
			fmt.Printf("  modified:   %s\n", f)
		}
	}

	if len(untracked) > 0 {
		fmt.Println("\nUntracked files:")
		for _, f := range untracked {
			fmt.Printf("  %s\n", f)
		}
	}

	if len(staged) == 0 && len(modified) == 0 && len(untracked) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}

	return nil
}

// Ensure os package is used (for future use)
var _ = os.Getenv
