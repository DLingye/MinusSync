package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var bare bool
	var syncOnly bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new MinusSync repository",
		Long: `Initialize a new MinusSync repository at the specified path.
If no path is given, the current directory is used.

使用 --sync-only 创建「同步模式」仓库，该模式仅记录文件修改状态
并通过 msync sync 与远程同步，不保留完整版本历史。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 {
				path = args[0]
			}

			_, err := repo.Init(repo.InitOptions{
				Path:     path,
				Bare:     bare,
				SyncOnly: syncOnly,
			})
			if err != nil {
				return err
			}

			if path == "" {
				path = "."
			}
			if syncOnly {
				fmt.Printf("Initialized sync-only MinusSync repository in %s/.msync/\n", path)
			} else {
				fmt.Printf("Initialized empty MinusSync repository in %s/.msync/\n", path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&bare, "bare", false, "Create a bare repository")
	cmd.Flags().BoolVar(&syncOnly, "sync-only", false, "Create a sync-only repository (no version history)")

	return cmd
}
