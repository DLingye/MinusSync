package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func mergeCmd() *cobra.Command {
	var noFF bool

	cmd := &cobra.Command{
		Use:   "merge <branch>",
		Short: "Merge a branch into the current branch",
		Long:  "Merge the specified branch into the current branch.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			return mergeBranch(r, args[0], noFF)
		},
	}

	cmd.Flags().BoolVar(&noFF, "no-ff", false, "Create a merge commit even if fast-forward is possible")
	return cmd
}

func mergeBranch(r *repo.Repository, branchName string, noFF bool) error {
	// Resolve current HEAD
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}

	// Resolve the branch to merge
	mergeHash, err := r.Refs.GetBranch(branchName)
	if err != nil {
		return fmt.Errorf("branch %q not found", branchName)
	}

	// Check if already merged
	if headHash.Equal(mergeHash) {
		fmt.Println("Already up to date.")
		return nil
	}

	// Find merge base
	baseHash, err := findMergeBase(r.ObjectsPath(), headHash, mergeHash)
	if err != nil {
		return fmt.Errorf("find merge base: %w", err)
	}

	// Check for fast-forward (HEAD is ancestor of mergeHash)
	if baseHash.Equal(headHash) && !noFF {
		// Fast-forward: just move HEAD to mergeHash
		currentBranch, _ := r.Refs.CurrentBranch(r.HeadPath())
		if currentBranch != "" {
			r.Refs.SetBranch(currentBranch, mergeHash)
		}
		// Checkout the new tree
		commit, err := object.ReadCommit(r.ObjectsPath(), mergeHash)
		if err != nil {
			return err
		}
		fmt.Printf("Fast-forward\n")
		return object.CheckoutTree(r.ObjectsPath(), commit.Tree, r.Path)
	}

	// Three-way merge
	headCommit, err := object.ReadCommit(r.ObjectsPath(), headHash)
	if err != nil {
		return err
	}
	mergeCommit, err := object.ReadCommit(r.ObjectsPath(), mergeHash)
	if err != nil {
		return err
	}

	// Perform merge
	if err := performMerge(r, headCommit.Tree, mergeCommit.Tree, baseHash); err != nil {
		return fmt.Errorf("merge conflict: %w", err)
	}

	// Create merge commit
	currentBranch, _ := r.Refs.CurrentBranch(r.HeadPath())
	if currentBranch == "" {
		return fmt.Errorf("detached HEAD: cannot merge")
	}

	// Build tree from working directory
	treeHash, err := object.BuildTree(r.ObjectsPath(), r.Path, func(p string) bool {
		return r.Ignore != nil && r.Ignore.IsIgnored(p, false)
	})
	if err != nil {
		return err
	}

	mergeCommitData := object.NewCommitData(
		treeHash,
		[]hash.Hash{headHash, mergeHash},
		r.Config.GetDefault("user", "name", "unknown"),
		r.Config.GetDefault("user", "email", "unknown@unknown"),
		"",
		fmt.Sprintf("Merge branch '%s'", branchName),
	)

	mergeCommitHash, err := object.WriteCommit(r.ObjectsPath(), mergeCommitData)
	if err != nil {
		return err
	}

	r.Refs.SetBranch(currentBranch, mergeCommitHash)
	fmt.Printf("Merge made: %s\n", mergeCommitHash.String())
	return nil
}

// findMergeBase finds the best common ancestor of two commits.
func findMergeBase(objectsDir string, a, b hash.Hash) (hash.Hash, error) {
	// Simple BFS from a, collecting ancestors, then BFS from b to find intersection
	seen := make(map[string]bool)
	queue := []hash.Hash{a}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if seen[current.Hex()] {
			continue
		}
		seen[current.Hex()] = true

		commit, err := object.ReadCommit(objectsDir, current)
		if err != nil {
			continue
		}
		queue = append(queue, commit.Parents...)
	}

	// BFS from b to find first ancestor in seen
	queue = []hash.Hash{b}
	seen2 := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if seen2[current.Hex()] {
			continue
		}
		seen2[current.Hex()] = true

		if seen[current.Hex()] {
			return current, nil
		}

		commit, err := object.ReadCommit(objectsDir, current)
		if err != nil {
			continue
		}
		queue = append(queue, commit.Parents...)
	}

	return hash.Zero, fmt.Errorf("no common ancestor found")
}

func performMerge(r *repo.Repository, oursTree, theirsTree, baseHash hash.Hash) error {
	// For now, do a simple union: checkout theirs over ours
	// A full three-way merge with conflict markers is implemented in internal/merge/
	return object.CheckoutTree(r.ObjectsPath(), theirsTree, r.Path)
}
