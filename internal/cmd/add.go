package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinusSync/internal/index"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func addCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "add [--all] [files...]",
		Short: "Stage files for commit",
		Long: `Add file contents to the staging area (index) for the next commit.
Use --all to stage all modified and new files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			if all {
				return addAll(r)
			}

			if len(args) == 0 {
				return fmt.Errorf("nothing specified, nothing added. Use --all to add all files")
			}

			for _, pattern := range args {
				matches, err := filepath.Glob(pattern)
				if err != nil || len(matches) == 0 {
					matches = []string{pattern}
				}
				for _, path := range matches {
					if err := addFile(r, path, true); err != nil {
						return fmt.Errorf("add %s: %w", path, err)
					}
				}
			}

			return r.SaveIndex()
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "A", false, "Stage all modified and new files")
	return cmd
}

func addFile(r *repo.Repository, path string, verbose bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	relPath, err := util.RelPath(r.Path, absPath)
	if err != nil {
		return err
	}

	// Normalize path
	relPath = util.NormalizePath(relPath)

	// Skip .msync directory and contents
	if strings.HasPrefix(relPath, ".msync/") || relPath == ".msync" {
		return nil
	}

	// Check if ignored
	if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
		return nil // Silently skip ignored files
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		// Add all files in directory
		return util.WalkDir(absPath, func(p string, fi os.FileInfo) error {
			if fi.IsDir() {
				base := filepath.Base(p)
				if base == ".msync" {
					return filepath.SkipDir
				}
				return nil
			}
			rp, _ := util.RelPath(r.Path, p)
			nrp := util.NormalizePath(rp)
			if strings.HasPrefix(nrp, ".msync/") || nrp == ".msync" {
				return nil
			}
			if r.Ignore != nil && r.Ignore.IsIgnored(nrp, false) {
				return nil
			}
			return addFile(r, p, verbose)
		})
	}

	// 读取文件内容（完整读取，用于创建 blob 对象）
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	// 计算部分哈希指纹（头+中+尾 4KB 采样）
	fingerprint := util.ComputeFingerprint(content)

	// 创建 blob 对象（全量哈希）
	h, err := object.WriteBlob(r.ObjectsPath(), content)
	if err != nil {
		return err
	}

	// Build index entry
	mtime := info.ModTime().UnixNano()
	entry := index.IndexEntry{
		Path:        relPath,
		Hash:        h,
		Fingerprint: fingerprint,
		Size:        info.Size(),
		MtimeNs:     mtime,
		Mode:        uint32(info.Mode()),
		Flags:       0,
	}

	r.Index.Add(entry)
	if verbose {
		fmt.Printf("staged: %s\n", relPath)
	}
	return nil
}

func addAll(r *repo.Repository) error {
	// Walk working directory and add all modified/new files
	cwd, _ := util.CurrentDir()
	return util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		relPath, err := util.RelPath(r.Path, path)
		if err != nil {
			return err
		}
		relPath = util.NormalizePath(relPath)

		if info.IsDir() {
			if relPath == ".msync" || strings.HasPrefix(relPath, ".msync/") {
				return filepath.SkipDir
			}
			if r.Ignore != nil && r.Ignore.IsIgnored(relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .msync contents
		if strings.HasPrefix(relPath, ".msync/") || relPath == ".msync" {
			return nil
		}

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		// Check if file is dirty (changed or new)
		dirty, _ := r.Index.Dirty(relPath, path, info)
		if !dirty && r.Index.IsTracked(relPath) {
			return nil
		}

		return addFile(r, path, true)
	})
}
