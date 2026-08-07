package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var bare bool
	var branch string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new MinusSync repository",
		Long: `Initialize a new MinusSync repository at the specified path.
If no path is given, the current directory is used.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 {
				path = args[0]
			}

			_, err := repo.Init(repo.InitOptions{
				Path:   path,
				Bare:   bare,
				Branch: branch,
			})
			if err != nil {
				return err
			}

			if path == "" {
				path = "."
			}
			fmt.Printf("Initialized empty MinusSync repository in %s/.msync/\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&bare, "bare", false, "Create a bare repository")
	cmd.Flags().StringVarP(&branch, "initial-branch", "b", "main", "Initial branch name")

	return cmd
}
