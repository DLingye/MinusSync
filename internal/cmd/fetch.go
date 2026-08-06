package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/sync"
	"github.com/spf13/cobra"
)

func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [remote]",
		Short: "Fetch objects and refs from a remote repository",
		Long:  "Download objects and refs from a remote repository without merging.",
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

			if err := sync.Fetch(r, remote); err != nil {
				return fmt.Errorf("fetch failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
