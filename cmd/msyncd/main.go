// msyncd is the MinusSync server daemon.
// It hosts multiple repositories and serves the MinusSync protocol over TCP/TLS.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MinusSync/internal/msyncd"
)

func main() {
	configPath := flag.String("config", "/etc/msyncd/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := msyncd.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and start server
	server, err := msyncd.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		server.Stop()
	}()

	log.Printf("msyncd %s starting on %s:%d", msyncd.Version, cfg.Listen.Address, cfg.Listen.Port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("msyncd stopped")
}
