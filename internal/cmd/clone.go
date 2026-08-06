package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/sync"
	"github.com/spf13/cobra"
)

func cloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <url> [dir]",
		Short: "Clone a remote repository",
		Long:  "Clone a remote MinusSync repository into a new directory.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			dir := ""
			if len(args) > 1 {
				dir = args[1]
			}

			_, err := sync.Clone(url, dir, "origin")
			if err != nil {
				return fmt.Errorf("clone failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
