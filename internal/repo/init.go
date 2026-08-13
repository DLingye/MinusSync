package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/index"
	"github.com/MinusSync/internal/util"
)

// InitOptions configures repository initialization.
type InitOptions struct {
	Path     string // Target directory
	Bare     bool   // Create a bare repository (no working tree)
	SyncOnly bool   // Create a sync-only repository (no version history)
}

// Init creates a new MinusSync repository.
func Init(opts InitOptions) (*Repository, error) {
	if opts.Path == "" {
		var err error
		opts.Path, err = util.CurrentDir()
		if err != nil {
			return nil, fmt.Errorf("get current dir: %w", err)
		}
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	root := absPath
	msyncPath := filepath.Join(root, MsyncDir)

	// Check if already initialized
	if info, err := os.Stat(msyncPath); err == nil && info.IsDir() {
		return nil, fmt.Errorf("repository already exists at %s", root)
	}

	// Create .msync directory
	if err := util.MkdirAll(msyncPath); err != nil {
		return nil, fmt.Errorf("create .msync: %w", err)
	}

	// Create subdirectories (no heads dir - single branch repository)
	dirs := []string{
		filepath.Join(msyncPath, ObjectsDir),
		filepath.Join(msyncPath, PackDir),
		filepath.Join(msyncPath, RefsDir, TagsDir),
		filepath.Join(msyncPath, RefsDir, RemotesDir),
		filepath.Join(msyncPath, SearchDir),
	}
	for _, d := range dirs {
		if err := util.MkdirAll(d); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Create HEAD file pointing to zero hash (no commits yet)
	headPath := filepath.Join(msyncPath, HeadFile)
	if err := os.WriteFile(headPath, []byte(hash.Zero.Hex()+"\n"), 0644); err != nil {
		return nil, fmt.Errorf("create HEAD: %w", err)
	}

	// Open the repo to set up config via the config package
	repo, err := openRepo(msyncPath)
	if err != nil {
		return nil, fmt.Errorf("open new repo: %w", err)
	}

	// Set default configuration
	repo.Config.Set("core", "bare", fmt.Sprintf("%v", opts.Bare))
	repo.Config.Set("core", "sync-only", fmt.Sprintf("%v", opts.SyncOnly))
	repo.Config.Set("core", "compression", "zstd")
	repo.Config.Set("core", "compression-level", "3")
	repo.Config.Set("core", "version", "1")
	if err := repo.Config.Save(); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	// Create empty index
	idx := index.New(repo.IndexPath())
	if err := idx.Save(); err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}
	repo.Index = idx

	return repo, nil
}
