package cmd

import (
	"fmt"
	"strings"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func logCmd() *cobra.Command {
	var oneline bool
	var graph bool
	var maxCount int
	var showAll bool

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show commit history",
		Long:  "Show the commit history log.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
			if err != nil {
				return err
			}

			return showLog(r, headHash, oneline, graph, maxCount, showAll)
		},
	}

	cmd.Flags().BoolVar(&oneline, "oneline", false, "Compact one-line format")
	cmd.Flags().BoolVar(&graph, "graph", false, "Show ASCII graph of branch history")
	cmd.Flags().IntVarP(&maxCount, "max-count", "n", 0, "Limit number of commits")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all branches")
	return cmd
}

func showLog(r *repo.Repository, startHash hash.Hash, oneline, graph bool, maxCount int, showAll bool) error {
	// Walk commits from HEAD following parent pointers
	current := startHash
	count := 0

	for !current.IsZero() {
		if maxCount > 0 && count >= maxCount {
			break
		}

		commit, err := object.ReadCommit(r.ObjectsPath(), current)
		if err != nil {
			return fmt.Errorf("read commit %s: %w", current.String(), err)
		}

		if oneline {
			msg := commit.Message
			if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
				msg = msg[:idx]
			}
			if graph {
				fmt.Printf("* ")
			}
			fmt.Printf("%s %s\n", current.String(), msg)
		} else {
			fmt.Printf("commit %s\n", current.Hex())
			if len(commit.Parents) > 1 {
				fmt.Printf("Merge:")
				for _, p := range commit.Parents {
					fmt.Printf(" %s", p.String())
				}
				fmt.Println()
			}
			fmt.Printf("Author: %s <%s>\n", commit.Author, commit.Email)
			fmt.Printf("Date:   %s\n", object.FormatTimestamp(commit.Timestamp))
			fmt.Printf("\n    %s\n\n", strings.ReplaceAll(commit.Message, "\n", "\n    "))
		}

		count++

		// Follow first parent (for simple history)
		if len(commit.Parents) > 0 {
			current = commit.Parents[0]
		} else {
			break
		}
	}

	return nil
}
