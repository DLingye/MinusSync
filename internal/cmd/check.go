package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/index"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/MinusSync/internal/util"
	"github.com/spf13/cobra"
)

func checkCmd() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "完整检查工作区文件的完整性",
		Long: `对工作区中所有已跟踪文件执行完整哈希校验，检测内容是否与索引记录一致。

与 status 不同，check 会对每个文件做全量 SHA-256 哈希，即使 size/mtime
未变化也能发现内容被篡改的文件（例如原地修改且保留了时间戳）。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			return runCheck(r, fix)
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "更新索引以反映实际文件状态")
	return cmd
}

func runCheck(r *repo.Repository, fix bool) error {
	if r.Index.Len() == 0 {
		fmt.Println("没有已跟踪的文件")
		return nil
	}

	checked := 0
	var modified, missing []string

	for _, entry := range r.Index.Entries {
		absPath := filepath.Join(r.Path, entry.Path)
		checked++

		// 文件是否还存在
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			missing = append(missing, entry.Path)
			continue
		}

		// 完整哈希（全量读取）
		content, err := os.ReadFile(absPath)
		if err != nil {
			missing = append(missing, entry.Path)
			continue
		}

		// 计算 blob 对象哈希（含 "blob <size>\0" 头部），与索引中存储的哈希一致
		blobData := object.Format(object.TypeBlob, content)
		actualHash := hash.Compute(blobData)

		// 与索引中记录的全量哈希对比
		if !actualHash.Equal(entry.Hash) {
			modified = append(modified, entry.Path)

			if fix {
				// 重新计算指纹并写入 blob，更新索引
				fingerprint := util.ComputeFingerprint(content)
				blobHash, err := object.WriteBlob(r.ObjectsPath(), content)
				if err != nil {
					return fmt.Errorf("写入 blob 失败 %s: %w", entry.Path, err)
				}

				newEntry := index.IndexEntry{
					Path:        entry.Path,
					Hash:        blobHash,
					Fingerprint: fingerprint,
					Size:        info.Size(),
					MtimeNs:     info.ModTime().UnixNano(),
					Mode:        uint32(info.Mode()),
					Flags:       entry.Flags,
				}
				r.Index.Add(newEntry)
			}
		}
	}

	// 输出结果
	fmt.Printf("已检查 %d 个文件\n", checked)

	if len(modified) > 0 {
		fmt.Printf("\n发现 %d 个内容被修改的文件：\n", len(modified))
		for _, f := range modified {
			fmt.Printf("  %s: %s\n", red("modified"), f)
		}
	}

	if len(missing) > 0 {
		fmt.Printf("\n发现 %d 个缺失的文件：\n", len(missing))
		for _, f := range missing {
			fmt.Printf("  %s: %s\n", yellow("missing"), f)
		}
	}

	if len(modified) == 0 && len(missing) == 0 {
		fmt.Println("所有文件与索引记录一致")
	} else if fix {
		if err := r.SaveIndex(); err != nil {
			return err
		}
		fmt.Println("\n索引已更新")
	} else if len(modified) > 0 {
		fmt.Println("\n提示：使用 --fix 更新索引，或使用 msync add 重新暂存")
	}

	return nil
}
