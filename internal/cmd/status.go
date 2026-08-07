package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		Long:  "Show the status of the working tree compared to the index and HEAD.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			return showStatus(r, short)
		},
	}

	cmd.Flags().BoolVarP(&short, "short", "s", false, "Short format output")
	return cmd
}

// fileChange holds info about a changed file and its line delta.
type fileChange struct {
	path      string
	additions int
	deletions int
	isNew     bool
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := 1
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	// Don't count trailing empty line
	if len(data) > 0 && data[len(data)-1] == '\n' {
		n--
	}
	return n
}

func computeDelta(r *repo.Repository, oldHash, newHash hash.Hash) (add, del int) {
	if newHash.IsZero() {
		return 0, 0
	}
	newContent, err := object.ReadBlob(r.ObjectsPath(), newHash)
	if err != nil {
		return 0, 0
	}
	newLines := countLines(newContent)

	if oldHash.IsZero() {
		return newLines, 0
	}
	oldContent, err := object.ReadBlob(r.ObjectsPath(), oldHash)
	if err != nil {
		return newLines, 0
	}
	oldLines := countLines(oldContent)

	add = newLines - oldLines
	if add < 0 {
		add = 0
	}
	del = oldLines - newLines
	if del < 0 {
		del = 0
	}
	return
}

func showStatus(r *repo.Repository, short bool) error {
	// Get the HEAD commit and tree for comparison
	var headTreeEntries map[string]object.TreeEntry
	headHash, err := r.Refs.ResolveHEAD(r.HeadPath())
	if err == nil && !headHash.IsZero() {
		headCommit, err := object.ReadCommit(r.ObjectsPath(), headHash)
		if err == nil {
			entries, err := object.ReadTree(r.ObjectsPath(), headCommit.Tree)
			if err == nil {
				headTreeEntries = make(map[string]object.TreeEntry)
				for _, e := range entries {
					headTreeEntries[e.Name] = e
				}
			}
		}
	}
	if headTreeEntries == nil {
		headTreeEntries = make(map[string]object.TreeEntry)
	}

	// --- Phase 1: Index vs HEAD → "staged" changes ---
	var stagedNew, stagedModified, stagedDeleted []fileChange

	for _, entry := range r.Index.Entries {
		headEntry, inHead := headTreeEntries[entry.Path]
		if !inHead {
			add, del := computeDelta(r, hash.Hash{}, entry.Hash)
			stagedNew = append(stagedNew, fileChange{path: entry.Path, additions: add, deletions: del, isNew: true})
		} else if !headEntry.Hash.Equal(entry.Hash) {
			add, del := computeDelta(r, headEntry.Hash, entry.Hash)
			stagedModified = append(stagedModified, fileChange{path: entry.Path, additions: add, deletions: del})
		}
	}

	for path := range headTreeEntries {
		if r.Index.Find(path) == nil {
			headEntry := headTreeEntries[path]
			content, _ := object.ReadBlob(r.ObjectsPath(), headEntry.Hash)
			del := countLines(content)
			stagedDeleted = append(stagedDeleted, fileChange{path: path, deletions: del})
		}
	}

	// --- Phase 2: Working tree vs Index → "unstaged" changes ---
	var modified, deleted, untracked []fileChange

	cwd, _ := util.CurrentDir()
	util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		relPath, _ := util.RelPath(r.Path, path)
		relPath = util.NormalizePath(relPath)

		if info.IsDir() {
			if filepath.Base(path) == ".msync" || strings.Contains(relPath, "/.msync/") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.Contains(relPath, "/.msync/") || strings.HasPrefix(relPath, ".msync/") {
			return nil
		}

		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		entry := r.Index.Find(relPath)
		if entry != nil {
			dirty, _ := r.Index.Dirty(relPath, info)
			if dirty {
				// Compute wip vs index delta by re-reading the file
				content, _ := os.ReadFile(path)
				newLines := countLines(content)
				oldLines := 0
				if !entry.Hash.IsZero() {
					oldContent, _ := object.ReadBlob(r.ObjectsPath(), entry.Hash)
					oldLines = countLines(oldContent)
				}
				add := newLines - oldLines
				del := oldLines - newLines
				if add < 0 {
					add = 0
				}
				if del < 0 {
					del = 0
				}
				modified = append(modified, fileChange{path: relPath, additions: add, deletions: del})
			}
		} else {
			if _, inHead := headTreeEntries[relPath]; inHead {
				modified = append(modified, fileChange{path: relPath})
			} else {
				untracked = append(untracked, fileChange{path: relPath})
			}
		}

		return nil
	})

	// Check for files in index but deleted from disk
	for _, entry := range r.Index.Entries {
		absPath := filepath.Join(r.Path, entry.Path)
		if !util.FileExists(absPath) {
			content, _ := object.ReadBlob(r.ObjectsPath(), entry.Hash)
			del := countLines(content)
			deleted = append(deleted, fileChange{path: entry.Path, deletions: del})
		}
	}

	// --- Display results ---
	hasChanges := false

	// Staged changes (green for labels, white for filenames)
	if len(stagedNew) > 0 || len(stagedModified) > 0 || len(stagedDeleted) > 0 {
		hasChanges = true
		fmt.Println("Changes to be committed:")
		fmt.Println("  (use \"msync commit\" to record these changes)")
		fmt.Println()
		for _, f := range stagedNew {
			fmt.Printf("  %s:   %s\n", green("new file"), green(f.path))
		}
		for _, f := range stagedModified {
			delta := formatDelta(f.additions, f.deletions)
			fmt.Printf("  %s:   %s%s\n", green("modified"), green(f.path), delta)
		}
		for _, f := range stagedDeleted {
			delta := formatDelta(0, f.deletions)
			fmt.Printf("  %s:    %s%s\n", green("deleted"), green(f.path), delta)
		}
		fmt.Println()
	}

	// Unstaged changes (red for labels)
	if len(modified) > 0 || len(deleted) > 0 {
		hasChanges = true
		fmt.Println("Changes not staged for commit:")
		fmt.Println("  (use \"msync add <file>...\" to update what will be committed)")
		fmt.Println()
		for _, f := range modified {
			delta := formatDelta(f.additions, f.deletions)
			fmt.Printf("  %s:   %s%s\n", red("modified"), red(f.path), delta)
		}
		for _, f := range deleted {
			delta := formatDelta(0, f.deletions)
			fmt.Printf("  %s:    %s%s\n", red("deleted"), red(f.path), delta)
		}
		fmt.Println()
	}

	// Untracked files (cyan for filenames)
	if len(untracked) > 0 {
		hasChanges = true
		fmt.Println("Untracked files:")
		fmt.Println("  (use \"msync add <file>...\" to include in what will be committed)")
		fmt.Println()
		for _, f := range untracked {
			fmt.Printf("  %s\n", cyan(f.path))
		}
		fmt.Println()
	}

	if !hasChanges {
		fmt.Println("nothing to commit, working tree clean")
	}

	return nil
}

func formatDelta(add, del int) string {
	if add == 0 && del == 0 {
		return ""
	}
	parts := []string{}
	if add > 0 {
		parts = append(parts, green(fmt.Sprintf("+%d", add)))
	}
	if del > 0 {
		parts = append(parts, red(fmt.Sprintf("-%d", del)))
	}
	return " " + strings.Join(parts, " ")
}

var _ = os.Getenv
