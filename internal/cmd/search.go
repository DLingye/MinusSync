package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/search"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var useRegex bool
	var glob string
	var maxResults int
	var caseSensitive bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search tracked files",
		Long:  "Search tracked files in the repository using full-text or regex search.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			// Resolve HEAD to get current tree
			headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
			if err != nil {
				return fmt.Errorf("no HEAD commit to search")
			}

			commit, err := object.ReadCommit(r.ObjectsPath(), headHash)
			if err != nil {
				return err
			}

			// Build or update search index
			idx, err := search.Open(r.MsyncPath + "/search")
			if err != nil {
				return err
			}
			defer idx.Close()

			if err := idx.Update(r.ObjectsPath(), commit.Tree); err != nil {
				return fmt.Errorf("build search index: %w", err)
			}

			// Execute query
			q := search.ParseQuery(query, caseSensitive, maxResults)
			_ = useRegex
			_ = glob

			results, err := idx.Search(q)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}

			for _, result := range results {
				fmt.Printf("%s (score: %.2f)\n", result.File, result.Score)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&useRegex, "regex", false, "Use regex pattern")
	cmd.Flags().StringVar(&glob, "glob", "", "Filter by file glob pattern")
	cmd.Flags().IntVarP(&maxResults, "max-results", "n", 20, "Maximum number of results")
	cmd.Flags().BoolVar(&caseSensitive, "case-sensitive", false, "Case sensitive search")
	return cmd
}
