package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func commitCmd() *cobra.Command {
	var message string
	var all bool
	var author string

	cmd := &cobra.Command{
		Use:   "commit -m <message>",
		Short: "Create a new commit",
		Long:  `Record changes to the repository by creating a new commit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				return fmt.Errorf("commit message required (-m)")
			}

			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			if all {
				if err := addAll(r); err != nil {
					return err
				}
			}

			return createCommit(r, message, author)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Automatically stage all modified files")
	cmd.Flags().StringVar(&author, "author", "", "Override author name")
	cmd.MarkFlagRequired("message")

	return cmd
}

func createCommit(r *repo.Repository, message, authorOverride string) error {
	if r.Index.Len() == 0 {
		return fmt.Errorf("nothing to commit (use 'msync add' to stage files)")
	}

	// Build tree from index entries
	treeHash, err := buildTreeFromIndex(r)
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}

	// Get parent commit (current HEAD)
	var parents []hash.Hash
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err == nil && !headHash.IsZero() {
		parents = []hash.Hash{headHash}
	}

	// Get author info
	authorName := r.Config.GetDefault("user", "name", "unknown")
	authorEmail := r.Config.GetDefault("user", "email", "unknown@unknown")
	if authorOverride != "" {
		authorName = authorOverride
	}

	hostname := util.Hostname()

	// Create commit
	commitData := object.NewCommitData(treeHash, parents, authorName, authorEmail, hostname, message)
	commitHash, err := object.WriteCommit(r.ObjectsPath(), commitData)
	if err != nil {
		return fmt.Errorf("write commit: %w", err)
	}

	// Update HEAD ref
	branchName, err := r.Refs.CurrentBranch(r.HeadPath())
	if err != nil {
		return err
	}
	if branchName == "" {
		return fmt.Errorf("detached HEAD: cannot commit (use 'msync checkout -b <branch>' first)")
	}

	if err := r.Refs.SetBranch(branchName, commitHash); err != nil {
		return fmt.Errorf("update branch: %w", err)
	}

	// Save index
	if err := r.SaveIndex(); err != nil {
		return err
	}

	fmt.Printf("[%s] %s\n", commitHash.String(), message)
	return nil
}

func buildTreeFromIndex(r *repo.Repository) (hash.Hash, error) {
	// Group index entries by directory and build trees bottom-up
	// For simplicity, build a flat tree with all entries
	var entries []object.TreeEntry
	for _, e := range r.Index.Entries {
		entries = append(entries, object.TreeEntry{
			Mode: e.Mode,
			Name: e.Path,
			Hash: e.Hash,
			Type: object.TypeBlob,
		})
	}

	treeContent := object.SerializeTree(entries)
	return object.Write(r.ObjectsPath(), object.Format(object.TypeTree, treeContent))
}
