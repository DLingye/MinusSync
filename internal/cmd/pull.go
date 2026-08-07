package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/sync"
	"github.com/spf13/cobra"
)

func pullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull [remote] [branch]",
		Short: "Pull changes from a remote repository",
		Long:  "Fetch and merge changes from a remote repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			remote := "origin"
			if len(args) > 0 {
				remote = args[0]
			}

			branch, _ := r.Refs.CurrentBranch(r.HeadPath())
			if len(args) > 1 {
				branch = args[1]
			}

			if err := sync.Pull(r, remote, branch); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
