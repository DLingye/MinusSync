// Package sync provides remote repository synchronization.
package sync

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MinusSync/internal/config"
)

// RemoteRef holds the parsed information about a remote.
type RemoteRef struct {
	Name string
	URL  string
	Host string
	Port int
	Path string
}

// ParseURL parses a remote URL into a RemoteRef.
// Supported formats:
//   - host:port
//   - msync://host:port/path
//   - host:port/path
func ParseURL(raw string) (*RemoteRef, error) {
	// Remove msync:// prefix if present
	raw = strings.TrimPrefix(raw, "msync://")

	// Split host:port from path
	parts := strings.SplitN(raw, "/", 2)
	hostPort := parts[0]
	path := "/"
	if len(parts) > 1 {
		path = "/" + parts[1]
	}

	// Parse host:port
	host := hostPort
	port := 65530 // Default msync port

	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		host = hostPort[:idx]
		p, err := strconv.Atoi(hostPort[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid port in %q: %w", raw, err)
		}
		port = p
	}

	return &RemoteRef{
		URL:  raw,
		Host: host,
		Port: port,
		Path: path,
	}, nil
}

// GetRemote returns the parsed remote configuration for a named remote.
func GetRemote(cfg *config.Config, name string) (*RemoteRef, error) {
	section := fmt.Sprintf("remote \"%s\"", name)
	url := cfg.Get(section, "url")
	if url == "" {
		return nil, fmt.Errorf("remote %q not found (no url)", name)
	}

	ref, err := ParseURL(url)
	if err != nil {
		return nil, err
	}
	ref.Name = name
	return ref, nil
}

// FormatURL formats a host:port as a URL string.
func FormatURL(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
