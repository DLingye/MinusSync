package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func branchCmd() *cobra.Command {
	var deleteFlag bool
	var moveFlag string

	cmd := &cobra.Command{
		Use:   "branch [--list] [-d <name>] [-m <old> <new>] [name]",
		Short: "List, create, or delete branches",
		Long:  "Manage branches in the repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			// Handle rename
			if moveFlag != "" {
				if len(args) < 1 {
					return fmt.Errorf("new branch name required for rename")
				}
				return renameBranch(r, moveFlag, args[0])
			}

			// Handle delete
			if deleteFlag {
				if len(args) < 1 {
					return fmt.Errorf("branch name required for delete")
				}
				return r.Refs.DeleteBranch(args[0])
			}

			// Create branch
			if len(args) > 0 {
				name := args[0]
				headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
				if err != nil {
					return fmt.Errorf("no HEAD commit: %w", err)
				}
				return r.Refs.SetBranch(name, headHash)
			}

			// List branches
			currentBranch, _ := r.Refs.CurrentBranch(r.HeadPath())
			branches, err := r.Refs.ListBranches()
			if err != nil {
				return err
			}
			for _, b := range branches {
				if b == currentBranch {
					fmt.Printf("* %s\n", b)
				} else {
					fmt.Printf("  %s\n", b)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteFlag, "delete", "d", false, "Delete a branch")
	cmd.Flags().StringVarP(&moveFlag, "move", "m", "", "Rename a branch")
	return cmd
}

func renameBranch(r *repo.Repository, oldName, newName string) error {
	hash, err := r.Refs.GetBranch(oldName)
	if err != nil {
		return fmt.Errorf("branch %q not found", oldName)
	}
	if err := r.Refs.SetBranch(newName, hash); err != nil {
		return err
	}
	return r.Refs.DeleteBranch(oldName)
}
