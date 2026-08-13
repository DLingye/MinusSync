package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/sync"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [remote]",
		Short: "同步模式：检测并同步修改的文件",
		Long: `扫描工作区，通过快速哈希校验（部分指纹）检测哪些文件被修改，
记录修改状态并与远程仓库同步。仅适用于 sync-only 模式。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			if !r.IsSyncOnly() {
				return fmt.Errorf("当前仓库不是 sync-only 模式，请使用 add/commit/push 完成版本管理")
			}

			remote := "origin"
			if len(args) > 0 {
				remote = args[0]
			}

			return runSync(r, remote)
		},
	}

	return cmd
}

func runSync(r *repo.Repository, remoteName string) error {
	// 1. 扫描工作区，用指纹快速检测修改
	added, modified, deleted := scanSyncChanges(r)

	// 2. 无修改时直接返回
	if len(added) == 0 && len(modified) == 0 && len(deleted) == 0 {
		fmt.Println("没有文件被修改")
		return nil
	}

	// 3. 打印修改列表（记录有哪些文件被修改）
	fmt.Println("检测到以下文件变更：")
	for _, f := range added {
		fmt.Printf("  %s: %s\n", green("new file"), green(f))
	}
	for _, f := range modified {
		fmt.Printf("  %s: %s\n", red("modified"), red(f))
	}
	for _, f := range deleted {
		fmt.Printf("  %s:  %s\n", yellow("deleted"), yellow(f))
	}
	fmt.Println()

	// 4. 对新增/修改文件创建 blob 并更新 index 快照
	for _, f := range added {
		if err := addFile(r, filepath.Join(r.Path, f), false); err != nil {
			return fmt.Errorf("add %s: %w", f, err)
		}
	}
	for _, f := range modified {
		if err := addFile(r, filepath.Join(r.Path, f), false); err != nil {
			return fmt.Errorf("add %s: %w", f, err)
		}
	}

	// 5. 删除的文件从 index 移除
	for _, f := range deleted {
		r.Index.Remove(f)
	}

	// 6. 保存 index
	if err := r.SaveIndex(); err != nil {
		return err
	}

	// 7. 自动 commit（sync-only 模式无需手动 commit）
	if err := createCommit(r, "sync", ""); err != nil {
		return fmt.Errorf("自动提交失败: %w", err)
	}

	// 8. 如果有远程，推送
	if hasRemote(r, remoteName) {
		if err := sync.Push(r, remoteName); err != nil {
			return fmt.Errorf("推送到远程失败: %w", err)
		}
	}

	fmt.Println("同步完成")
	return nil
}

// scanSyncChanges 扫描工作区，返回新增、修改、删除的文件列表。
// 使用 stat + 部分指纹快速检测，未变化的文件不会读取完整内容。
func scanSyncChanges(r *repo.Repository) (added, modified, deleted []string) {
	cwd, _ := util.CurrentDir()

	// 记录工作区实际存在的文件
	found := make(map[string]bool)

	util.WalkDir(cwd, func(path string, info os.FileInfo) error {
		relPath, err := util.RelPath(r.Path, path)
		if err != nil {
			return nil
		}
		relPath = util.NormalizePath(relPath)

		// 跳过目录
		if info.IsDir() {
			if relPath == ".msync" || strings.HasPrefix(relPath, ".msync/") {
				return filepath.SkipDir
			}
			if r.Ignore != nil && r.Ignore.IsIgnored(relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过 .msync 内容
		if strings.HasPrefix(relPath, ".msync/") || relPath == ".msync" {
			return nil
		}

		// 跳过 ignored 文件
		if r.Ignore != nil && r.Ignore.IsIgnored(relPath, false) {
			return nil
		}

		found[relPath] = true

		entry := r.Index.Find(relPath)
		if entry == nil {
			// 索引中不存在 → 新增
			added = append(added, relPath)
		} else {
			// 用指纹快速检测是否修改
			dirty, _ := r.Index.Dirty(relPath, path, info)
			if dirty {
				modified = append(modified, relPath)
			}
		}
		return nil
	})

	// 检测删除：索引中存在但工作区已不存在
	for _, entry := range r.Index.Entries {
		if !found[entry.Path] {
			deleted = append(deleted, entry.Path)
		}
	}

	return
}

// hasRemote 报告是否存在指定的远程。
func hasRemote(r *repo.Repository, name string) bool {
	section := fmt.Sprintf("remote \"%s\"", name)
	return r.Config.Has(section, "url")
}
