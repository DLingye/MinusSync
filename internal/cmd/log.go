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
	var maxCount int

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

			return showLog(r, headHash, oneline, maxCount)
		},
	}

	cmd.Flags().BoolVar(&oneline, "oneline", false, "Compact one-line format")
	cmd.Flags().IntVarP(&maxCount, "max-count", "n", 0, "Limit number of commits")
	return cmd
}

func showLog(r *repo.Repository, startHash hash.Hash, oneline bool, maxCount int) error {
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
			if msg == "" {
				msg = "(no message)"
			}
			fmt.Printf("%s #%d %s\n", yellow(current.String()), commit.Sequence, msg)
		} else {
			fmt.Printf("%s %s (#%d)\n", yellow("commit"), current.Hex(), commit.Sequence)
			fmt.Printf("Author: %s <%s>\n", commit.Author, commit.Email)
			fmt.Printf("Date:   %s\n", object.FormatTimestamp(commit.Timestamp, commit.TZOffset))
			if commit.Message != "" {
				fmt.Printf("\n    %s\n", strings.ReplaceAll(commit.Message, "\n", "\n    "))
			}
			fmt.Println()
		}

		count++

		if len(commit.Parents) > 0 {
			current = commit.Parents[0]
		} else {
			break
		}
	}

	return nil
}
