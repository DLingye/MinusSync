# MinusSync (msync)

一个使用 Go 语言编写的类 git 和 lix 的版本管理与文件同步系统。

## 功能特性

- **版本控制**：完整的 VCS 功能，支持提交、分支、标签、合并、差异比较和日志
- **远程同步**：通过 TCP/TLS 进行推送、拉取、抓取和克隆
- **二进制增量同步**：基于内容定义的分块（FastCDC），高效传输二进制文件
- **语义搜索**：对已跟踪文件进行代码感知的全文搜索
- **跨平台**：支持 Linux、Windows 和 Android (Termux) — 纯 Go 实现，CGO_ENABLED=0
- **客户端与服务器**：内置 `msyncd` 守护进程，支持多仓库托管和 systemd 集成

## 快速开始

### 安装

```bash
# 从源码编译
git clone https://github.com/MinusSync/MinusSync.git
cd MinusSync
make build
sudo make install
```

### 基本使用

```bash
# 初始化仓库
msync init my-project
cd my-project

# 添加并提交文件
echo "hello world" > README.md
msync add README.md
msync commit -m "初始提交"

# 查看历史
msync log --oneline

# 创建分支
msync branch feature-x
msync checkout feature-x

# 修改并提交
echo "新功能" >> main.go
msync add -A
msync commit -m "添加功能 X"

# 合并回主分支
msync checkout main
msync merge feature-x

# 添加远程并推送
msync remote add origin myserver.com:65530
msync push origin main

# 搜索已跟踪文件
msync search "function parse"
```

### 服务器配置

```bash
# 创建配置
sudo mkdir -p /etc/msyncd
sudo cp contrib/systemd/msyncd@.service /usr/lib/systemd/system/

# 启动服务器
sudo systemctl enable --now msyncd@default
```

## 平台支持

| 平台 | 状态 |
|------|------|
| Linux (amd64) | ✅ 完全支持 |
| Linux (arm64) | ✅ 完全支持 (Termux/Android) |
| Windows (amd64) | ✅ 完全支持 |

## 编译

```bash
# 当前平台
make build

# 交叉编译所有目标
make build-all

# 运行测试
make test
```

## 命令列表

```
msync init        初始化仓库
msync clone       克隆远程仓库
msync add         暂存文件
msync commit      创建提交
msync status      查看工作区状态
msync log         提交历史
msync diff        显示变更
msync branch      分支管理
msync checkout    切换分支
msync merge       合并分支
msync tag         标签管理
msync remote      远程管理
msync push        推送到远程
msync pull        从远程拉取
msync fetch       从远程抓取
msync search      搜索已跟踪文件
msync gc          垃圾回收
msync fsck        完整性检查
msync config      获取/设置配置
msync serve       启动嵌入式服务器
msync version     打印版本
```

## 许可证

AGPL-3.0-or-later
