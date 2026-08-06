package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func checkoutCmd() *cobra.Command {
	var createBranch bool

	cmd := &cobra.Command{
		Use:   "checkout [-b] <branch|commit>",
		Short: "Switch branches or restore working tree files",
		Long:  "Switch to a branch or commit, updating the working tree.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			target := args[0]

			if createBranch {
				headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
				if err != nil {
					return fmt.Errorf("resolve HEAD: %w", err)
				}
				if err := r.Refs.SetBranch(target, headHash); err != nil {
					return err
				}
			}

			return checkoutRef(r, target)
		},
	}

	cmd.Flags().BoolVarP(&createBranch, "branch", "b", false, "Create and switch to a new branch")
	return cmd
}

func checkoutRef(r *repo.Repository, target string) error {
	var commitHash hash.Hash
	branchName := ""

	if r.Refs.HasBranch(target) {
		h, err := r.Refs.GetBranch(target)
		if err != nil {
			return err
		}
		// Update HEAD to point to branch (e.g., "refs/heads/main")
		refTarget := "refs/heads/" + target
		if err := r.Refs.WriteHeadSymbolic(r.HeadPath(), refTarget); err != nil {
			return err
		}
		commitHash = h
		branchName = target
	} else {
		// Try as commit hash (full or short)
		h, err := resolveHash(r.ObjectsPath(), target)
		if err != nil {
			return fmt.Errorf("branch or commit %q not found: %w", target, err)
		}
		commitHash = h
		// Detached HEAD
		if err := r.Refs.WriteHeadDetached(r.HeadPath(), h); err != nil {
			return err
		}
	}

	// Checkout the tree
	commit, err := object.ReadCommit(r.ObjectsPath(), commitHash)
	if err != nil {
		return fmt.Errorf("read commit: %w", err)
	}

	if err := object.CheckoutTree(r.ObjectsPath(), commit.Tree, r.Path); err != nil {
		return fmt.Errorf("checkout tree: %w", err)
	}

	if branchName != "" {
		fmt.Printf("Switched to branch '%s'\n", branchName)
	} else {
		fmt.Printf("HEAD is now at %s\n", commitHash.String())
	}

	return nil
}

// resolveHash resolves a full or short hex hash by scanning the object store.
func resolveHash(objectsDir, s string) (hash.Hash, error) {
	// Try full hash first
	if len(s) == hash.HexSize {
		return hash.FromHex(s)
	}
	// For short hashes, we'd need to scan the object store.
	// For now, only support full hashes.
	return hash.Zero, fmt.Errorf("short hash resolution not yet implemented (use full hash)")
}
