package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/gc"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func gcCmd() *cobra.Command {
	var prune bool
	var aggressive bool

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect unreachable objects",
		Long:  "Remove unreachable objects and optimize the object store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			report, err := gc.Collect(
				r.Path,
				r.ObjectsPath(),
				r.MsyncPath+"/refs",
				r.HeadPath(),
				prune || aggressive,
				true,
			)
			if err != nil {
				return err
			}

			fmt.Printf("Objects: %d total, %d reachable, %d unreachable\n",
				report.TotalObjects, report.ReachableObjects, report.UnreachableObjects)

			if report.Pruned {
				fmt.Printf("Pruned %d unreachable objects, recovered %d bytes\n",
					report.UnreachableObjects, report.RecoveredBytes)
			} else if report.UnreachableObjects > 0 {
				fmt.Println("Use --prune to remove unreachable objects")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&prune, "prune", false, "Prune unreachable objects")
	cmd.Flags().BoolVar(&aggressive, "aggressive", false, "Aggressive optimization")
	return cmd
}
