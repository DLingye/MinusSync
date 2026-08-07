// Package util provides cross-platform helper functions.
package util

import (
	"io"
	"os"
	"path/filepath"
)

// FileExists reports whether a file exists and is a regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// DirExists reports whether a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// MkdirAll creates a directory and all parents.
func MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

// WriteFile writes data to a file atomically by writing to a temp file and renaming.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// ReadFile reads an entire file.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := MkdirAll(filepath.Dir(dst)); err != nil {
		return err
	}

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return d.Close()
}

// RemoveAll removes a path and all its contents.
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// WalkDir walks a directory tree, calling fn for each file.
func WalkDir(root string, fn func(path string, info os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return fn(path, info)
	})
}

// IsSymlink reports whether a file is a symbolic link.
func IsSymlink(mode os.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

// ReadSymlink reads the target of a symbolic link.
func ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}
