package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/msyncd"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	var port int
	var repoDir string
	var tlsCert string
	var tlsKey string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an embedded MinusSync server",
		Long:  "Start the MinusSync protocol server for a single repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := msyncd.DefaultConfig()
			cfg.Listen.Port = port
			cfg.Listen.Address = "0.0.0.0"
			if tlsCert != "" {
				cfg.Listen.TLS.Enabled = true
				cfg.Listen.TLS.CertFile = tlsCert
				cfg.Listen.TLS.KeyFile = tlsKey
			}
			cfg.Repositories = append(cfg.Repositories, msyncd.RepositoryConfig{
				Name: "default",
				Path: repoDir,
			})

			server, err := msyncd.NewServer(cfg)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}

			fmt.Printf("Serving on :%d (repo: %s)\n", port, repoDir)
			return server.Start()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 65530, "Port to listen on")
	cmd.Flags().StringVar(&repoDir, "repo-dir", ".", "Repository directory")
	cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-key", "", "TLS key file")
	return cmd
}
