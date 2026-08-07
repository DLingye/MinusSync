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
		Use:   "commit [-m <message>]",
		Short: "Create a new commit",
		Long:  `Record changes to the repository by creating a new commit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	parentSeq := uint64(0)
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())

	// Check if anything actually changed compared to HEAD
	if err == nil && !headHash.IsZero() {
		headCommit, err := object.ReadCommit(r.ObjectsPath(), headHash)
		if err == nil && headCommit.Tree.Equal(treeHash) {
			return fmt.Errorf("nothing to commit, working tree clean")
		}
		if err == nil {
			parentSeq = headCommit.Sequence
		}
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
	commitData := object.NewCommitData(treeHash, parents, authorName, authorEmail, hostname, message, parentSeq)
	commitHash, err := object.WriteCommit(r.ObjectsPath(), commitData)
	if err != nil {
		return fmt.Errorf("write commit: %w", err)
	}

	// Update HEAD directly (single-branch model)
	if err := r.Refs.WriteHead(r.HeadPath(), commitHash); err != nil {
		return fmt.Errorf("update HEAD: %w", err)
	}

	// Save index
	if err := r.SaveIndex(); err != nil {
		return err
	}

	fmt.Printf("[%s #%d] %s\n", commitHash.String(), commitData.Sequence, message)
	return nil
}

func buildTreeFromIndex(r *repo.Repository) (hash.Hash, error) {
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
