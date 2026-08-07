package util

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath converts a path to use forward slashes, for consistent storage.
func NormalizePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// RepoPath joins elements with the repository root to form a path within .msync.
func RepoPath(root string, parts ...string) string {
	all := append([]string{root}, parts...)
	return filepath.Join(all...)
}

// IsAbs reports whether a path is absolute.
func IsAbs(path string) bool {
	return filepath.IsAbs(path)
}

// RelPath computes a relative path from base to target.
func RelPath(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// CurrentDir returns the current working directory.
func CurrentDir() (string, error) {
	return os.Getwd()
}

// SplitExt splits a path into its base name and extension.
// The extension includes the dot.
func SplitExt(path string) (string, string) {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	return base, ext
}

// IsHidden reports whether a file or directory should be considered hidden.
// On all platforms, paths starting with "." are hidden.
// On Windows, files with the hidden attribute are also hidden.
func IsHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// PathSeparator returns the OS-specific path separator as a string.
func PathSeparator() string {
	return string(filepath.Separator)
}
