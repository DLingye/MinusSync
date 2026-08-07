package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/sync"
	"github.com/spf13/cobra"
)

func pushCmd() *cobra.Command {
	var setUpstream bool
	var force bool

	cmd := &cobra.Command{
		Use:   "push [remote] [branch]",
		Short: "Push changes to a remote repository",
		Long:  "Push local commits to a remote repository.",
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

			branch, err := r.Refs.CurrentBranch(r.HeadPath())
			if err != nil || branch == "" {
				return fmt.Errorf("not on a branch: cannot push")
			}
			if len(args) > 1 {
				branch = args[1]
			}

			_ = setUpstream
			_ = force

			if err := sync.Push(r, remote, branch); err != nil {
				return fmt.Errorf("push failed: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&setUpstream, "set-upstream", "u", false, "Set upstream tracking")
	cmd.Flags().BoolVar(&force, "force", false, "Force push (allow non-fast-forward)")
	return cmd
}
