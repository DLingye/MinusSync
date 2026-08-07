// Package repo provides repository operations for MinusSync.
package repo

import (
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/config"
	"github.com/MinusSync/internal/index"
	"github.com/MinusSync/internal/ignore"
	"github.com/MinusSync/internal/refs"
)

// Layout constants for the .msync directory structure.
const (
	MsyncDir   = ".msync"
	ObjectsDir = "objects"
	PackDir    = "objects/pack"
	RefsDir    = "refs"
	TagsDir    = "refs/tags"
	RemotesDir = "refs/remotes"
	HeadFile   = "HEAD"
	IndexFile  = "index"
	ConfigFile = "config"
	IgnoreFile = ".msyncign"
	SearchDir  = "search"
)

// Repository represents an open MinusSync repository.
type Repository struct {
	Path      string          // Root of the working directory (or bare repo)
	MsyncPath string          // Path to the .msync directory
	Config    *config.Config
	Index     *index.Index
	Refs      *refs.Refs
	Ignore    *ignore.Matcher
	IsBare    bool
}

// Open opens an existing repository.
// It searches upward from the given path to find the .msync directory.
func Open(path string) (*Repository, error) {
	// Search for .msync directory
	msyncPath := findMsyncDir(path)
	if msyncPath == "" {
		return nil, &ErrNotRepo{Path: path}
	}

	return openRepo(msyncPath)
}

// openRepo opens a repository given the .msync path.
func openRepo(msyncPath string) (*Repository, error) {
	// Resolve to absolute paths
	absMsyncPath, err := filepath.Abs(msyncPath)
	if err != nil {
		return nil, err
	}
	msyncPath = absMsyncPath

	root := filepath.Dir(msyncPath)
	isBare := root == msyncPath || filepath.Base(root) == MsyncDir

	if isBare {
		root = msyncPath
	}

	r := &Repository{
		Path:      root,
		MsyncPath: msyncPath,
		IsBare:    isBare,
	}

	// Load config
	cfg, err := config.Load(msyncPathJoin(msyncPath, ConfigFile))
	if err != nil {
		return nil, err
	}
	r.Config = cfg

	// Load index
	idx, err := index.Load(msyncPathJoin(msyncPath, IndexFile))
	if err != nil {
		return nil, err
	}
	r.Index = idx

	// Initialize refs
	r.Refs = refs.New(msyncPathJoin(msyncPath, RefsDir))

	// Load ignore patterns (from working tree root)
	ignorePath := filepath.Join(root, IgnoreFile)
	ign, _ := ignore.Load(ignorePath)
	r.Ignore = ign

	return r, nil
}

// findMsyncDir searches upward from path to find .msync.
func findMsyncDir(path string) string {
	// Check if path itself is .msync
	if filepath.Base(path) == MsyncDir {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
	}

	// Check if path has .msync directly
	testPath := path
	for {
		msyncPath := filepath.Join(testPath, MsyncDir)
		info, err := os.Stat(msyncPath)
		if err == nil && info.IsDir() {
			return msyncPath
		}

		parent := filepath.Dir(testPath)
		if parent == testPath {
			return "" // Reached root
		}
		testPath = parent
	}
}

// msyncPathJoin joins path elements relative to the .msync directory.
func msyncPathJoin(msyncDir string, parts ...string) string {
	all := append([]string{msyncDir}, parts...)
	return filepath.Join(all...)
}

// ObjectsPath returns the path to the objects directory.
func (r *Repository) ObjectsPath() string {
	return msyncPathJoin(r.MsyncPath, ObjectsDir)
}

// HeadPath returns the path to the HEAD file.
func (r *Repository) HeadPath() string {
	return msyncPathJoin(r.MsyncPath, HeadFile)
}

// IndexPath returns the path to the index file.
func (r *Repository) IndexPath() string {
	return msyncPathJoin(r.MsyncPath, IndexFile)
}

// Close saves any pending state (index, config).
func (r *Repository) Close() error {
	if r.Index != nil {
		if err := r.Index.Save(); err != nil {
			return err
		}
	}
	if r.Config != nil {
		if err := r.Config.Save(); err != nil {
			return err
		}
	}
	return nil
}

// SaveConfig writes the configuration to disk.
func (r *Repository) SaveConfig() error {
	if r.Config != nil {
		return r.Config.Save()
	}
	return nil
}

// SaveIndex writes the index to disk.
func (r *Repository) SaveIndex() error {
	if r.Index != nil {
		return r.Index.Save()
	}
	return nil
}

// ErrNotRepo is returned when the path is not inside a MinusSync repository.
type ErrNotRepo struct {
	Path string
}

func (e *ErrNotRepo) Error() string {
	return "not a MinusSync repository: " + e.Path
}
