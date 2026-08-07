package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/sync"
	"github.com/spf13/cobra"
)

func pullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull [remote]",
		Short: "Pull changes from a remote repository",
		Long:  "Fetch changes from a remote repository and update the working tree.",
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

			if err := sync.Pull(r, remote); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
