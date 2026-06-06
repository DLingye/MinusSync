# Minus Sync (msync)  

用 C 语言编写的轻量级版本控制系统，模仿 Git 但去除了臃肿功能。单个二进制文件（~70KB），支持 **Linux**、**Windows**（MinGW/MSVC）和 **Android Termux**，同时具备客户端和服务端能力。

[English](README.md)

## 为什么选择 msync？

| 特性 | msync | Git |
|------|-------|-----|
| 暂存区 | **无** — 直接提交 | 有（`git add`） |
| 提交元数据 | 作者、邮箱、**主机名** | 作者、邮箱 |
| 对象哈希 | SHA-256 | SHA-1（正向 SHA-256 迁移） |
| 传输方式 | 内置 TCP（默认 `:65530`） | HTTP / SSH / Git 协议 |
| 服务端 | 内置（`msync serve`） | 独立守护进程（`git daemon`） |
| 二进制体积 | ~70 KB | 10+ MB |
| 单仓库多远程 | 支持，可命名 | 支持，可命名 |
| 镜像守护进程 | 内置（`msync mirror start`） | 需外部工具 |
| 图形化日志 | 内置（`msync log --graphic`） | `git log --graph` |
| 忽略文件 | `.msyncign`（glob 模式） | `.gitignore` |
| 完整性检测 | 内置（`msync fsck`） | `git fsck` |
| 垃圾回收 | 内置（`msync gc`） | `git gc` |

## 编译

```sh
# 依赖：GCC 或 Clang、GNU Make
make

# 可选：安装到系统
sudo make install PREFIX=/usr/local

# Windows（MinGW-w64）
make CC=gcc
# 输出：msync.exe
```

除 C 标准库和操作系统 socket 外，无任何外部依赖。

## 快速上手

```sh
# 1. 初始化仓库
msync init

# 2. 配置身份信息
msync config user.name  "张三"
msync config user.email "zhangsan@example.com"

# 3. 创建文件并查看状态
echo "hello" > README.md
msync status
# On branch master
# New files:
#   new:  README.md

# 4. 直接提交（无需暂存）
msync commit -m "初始提交"

# 5. 查看历史
msync log
msync log --oneline        # 每行一次提交
msync log --graphic         # 图形化分支历史
```

## 命令参考

### 仓库初始化

```sh
msync init                              # 初始化新仓库
msync config <key>                      # 读取配置值
msync config <key> <value>             # 设置配置值
```

### 日常操作

```sh
msync status                            # 显示新增、修改、删除的文件
msync commit -m <消息>                  # 提交所有变更（无暂存区）
msync log [-n <数量>]                   # 查看提交历史
msync log --oneline [-n <数量>]         # 紧凑一行模式
msync log --graphic [-n <数量>]          # 图形化分支历史
```

### 分支管理

```sh
msync branch                            # 列出所有分支
msync branch <名称>                     # 创建新分支
msync branch -d <名称>                  # 删除分支
msync checkout <分支>                   # 切换到分支
msync checkout <提交哈希>               # 分离 HEAD 到某次提交
```

### 远程仓库管理

```sh
msync remote add <名称> <地址>          # 添加命名远程仓库
msync remote list                       # 列出所有远程仓库
msync remote remove <名称>              # 移除远程仓库
```

### 远程同步

```sh
msync clone <地址> [目录]               # 克隆远程仓库
msync clone --name <名称> <地址> [目录]  # 克隆并自定义远程名
msync push   <远程|地址> [分支]          # 推送到远程（拒绝非快进推送）
msync update <远程|地址> [分支]          # 从远程拉取（自动合并）
```

### 服务端

```sh
msync serve [-p <端口>]                 # 启动服务端（默认端口 65530）
```

### 忽略文件

```sh
msync ignore add <模式>                 # 添加忽略规则
msync ignore list                       # 列出所有忽略规则
msync ignore remove <模式>              # 移除忽略规则
```

规则存储在 `.msyncign` 文件中。语法见 [.msyncign](#msyncign) 章节。

### 仓库维护

```sh
msync fsck [-v]                         # 验证仓库完整性
msync gc                                # 显示不可达对象
msync gc --prune                        # 删除不可达对象
```

### 仓库镜像

```sh
msync mirror once <源地址>              # 一次性全量镜像
msync mirror start <地址> [-i <秒>] [--serve <端口>]
                                        # 持续镜像守护进程

# 也可以通过配置文件设置
msync config mirror.source   主机:65530
msync config mirror.interval 300
msync config mirror.serve-port 65530
msync mirror start                      # 使用配置中的值
```

## 远程地址格式

```
主机:端口                       # 例如：192.168.1.100:65530
msync://主机:端口               # 协议前缀（可选）
localhost:65530                # 默认端口可省略 → localhost（使用 65530）
```

省略端口时默认使用 `65530`。

## 提交记录

每次提交存储以下信息：

```
tree <sha256哈希值>
parent <sha256哈希值>          （合并提交会有两条 parent 行）
author <姓名> <邮箱> <Unix时间戳>
hostname <机器主机名>

<提交消息>
```

`hostname` 字段用于区分同一作者在不同设备上的提交——当同一人使用多台设备协作时非常有用。

## 仓库目录结构

```
.msync/
├── HEAD                    # 当前分支（例如 "ref: refs/heads/master"）
├── config                  # INI 格式配置文件
├── index                   # 二进制文件索引，快速状态检测
├── objects/                # 基于 SHA-256 的内容寻址对象存储
│   └── XX/
│       └── XXXX...         # 按哈希前缀存储的对象
└── refs/
    ├── heads/              # 本地分支
    │   ├── master
    │   └── feature-x
    └── remotes/            # 远程追踪引用
        ├── origin/
        │   └── master
        └── mirror/
            └── heads/
                └── master
```

### 对象类型

| 类型 | 格式 | 用途 |
|------|------|------|
| blob | `blob <大小>\0<内容>` | 文件内容 |
| tree | `tree <大小>\0<模式> <名称>\0<哈希>...` | 目录结构 |
| commit | `commit <大小>\0<元数据>\n\n<消息>` | 提交快照 |

### 索引格式

二进制文件，以魔数 `MSYN` 开头，后跟版本号和条目数量，每个条目包含：文件路径、SHA-256 哈希、大小、修改时间、权限模式。通过大小/修改时间快速比对（哈希校验作为回退），实现 O(1) 级别的变更检测。

## 网络协议

基于 TCP 的 simple text-command / binary-data 协议：

```
客户端                              服务端
  │                                   │
  ├── LIST ──────────────────────────►│  列出所有引用
  │◄─ <数量>                         │
  │◄─ <哈希> <引用名>                │  （每条引用）
  │── OK ───────────────────────────►│
  │                                   │
  ├── FETCH ─────────────────────────►│  获取对象
  ├── WANT <哈希>                     │
  ├── HAVE <哈希>                     │  （可选，增量获取）
  ├── DONE ──────────────────────────►│
  │◄─ PACK <数量>                    │  二进制包跟随
  │◄─ [哈希][类型][大小][数据]       │  （每个对象 32+1+4+N 字节）
  │── OK ───────────────────────────►│
  │                                   │
  ├── PUSH ──────────────────────────►│  推送对象
  ├── UPDATE <引用> <旧哈希> <新哈希>►│  服务端校验旧哈希
  │◄─ WANT_PACK / ERR ──────────────│  非快进推送 → ERR
  ├── PACK <数量>                     │
  ├── [哈希][类型][大小][数据]       │
  │◄─ OK ────────────────────────────│
```

## 配置参考

```ini
[user]
    name = 张三
    email = zhangsan@example.com

[remote "origin"]
    url = server.example.com:65530

[remote "backup"]
    url = backup.example.com:65530

[mirror]
    source = primary-server:65530
    interval = 300
    serve-port = 65530
```

## 工作流示例

### 单人本地开发

```sh
msync init
msync config user.name "张三"
echo "项目开始" > main.c
msync commit -m "初始提交"
# 开发，开发……
msync commit -m "添加功能 X"
msync log --oneline
```

### 团队协作（中央服务器模式）

```sh
# 张三：启动服务端
张三$ msync serve -p 65530

# 李四：克隆并开发
李四$ msync clone zhangsan-server:65530 project
李四$ cd project
李四$ msync config user.name "李四"
李四$ echo "李四的代码" >> main.c
李四$ msync commit -m "李四的功能"
李四$ msync push origin

# 张三：拉取李四的变更
张三$ msync update origin
```

### 分支工作流

```sh
msync checkout -b feature-x
# 开发功能……
msync commit -m "功能 X 开发中"
msync checkout master
# 修复紧急 bug……
msync commit -m "紧急修复"
msync checkout feature-x
msync update origin            # 集成 master 的修复
msync commit -m "功能 X 完成"
msync checkout master
# 合并（通过从 feature-x 的远端 update，或手动合并）
```

### 镜像备份 / 分布式团队

```sh
# 在备份服务器上搭建镜像
backup$ msync init
backup$ msync mirror start primary-server:65530 -i 300 --serve 65530

# 镜像每5分钟同步所有分支，同时提供只读服务
# 备份区域的团队成员从镜像克隆
dev$ msync clone backup-server:65530 project
```

### 多远程仓库协同

```sh
# 一个本地仓库同时关联多个远程
msync remote add origin   main-server:65530
msync remote add backup   backup-server:65530
msync remote add upstream upstream:65530

msync remote list
# Name                 URL
# ----                 ---
# origin               main-server:65530
# backup               backup-server:65530
# upstream             upstream:65530

msync push origin master        # 推送到主服务器
msync push backup master        # 同时推送到备份服务器
msync update upstream master    # 从上游拉取更新
```

## 冲突处理

### Push 冲突（非快进）

当远程在你上次同步后有新的提交时，push 会被拒绝：

```sh
$ msync push origin
Push rejected: the remote has newer commits.
Run 'msync update origin' first to integrate remote changes.
```

服务端会校验客户端发来的 `old_hash` 是否与当前引用一致。

### Update 合并

- **快进合并**：远程是本地提交的直接后代 — 自动更新。
- **分叉历史**：本地和远程各自有独立提交 — msync 找到共同祖先，创建合并提交，保留双方历史。
- **无法自动合并**：警告用户，使用远程版本作为基础，本地独有的提交保留在历史中可回溯。

## .msyncign

`.msyncign` 文件（兼容 `.gitignore` 语法）用于指定不需要追踪的文件。

### 模式语法

| 语法 | 说明 | 示例 |
|------|------|------|
| `*.ext` | 通配符匹配 | `*.o` 匹配 `main.o`、`src/util.o` |
| `dir/` | 目录匹配 | `build/` 忽略整个 `build/` 目录 |
| `path/*.ext` | 路径限定匹配 | `src/*.o` 匹配 `src/main.o` 但不匹配 `lib/src/a.o` |
| `!pattern` | 取反（不忽略） | `!important.o` 从通配规则中排除 `important.o` |
| `# comment` | 注释行 | `# 编译产物` |
| `**` | 递归通配 | `**/temp` 匹配 `temp`、`a/temp`、`a/b/temp` |

### 示例

```
# 编译产物
*.o
*.exe
build/

# 依赖目录
node_modules/

# 环境配置
.env
*.log

# 但保留这个文件
!important.log
```

### 管理命令

```sh
msync ignore add "*.o"          # 添加规则
msync ignore list               # 查看所有规则
msync ignore remove "*.o"      # 移除规则
```

## 仓库维护

### 完整性检测（`msync fsck`）

验证仓库是否完整，逐级检查：

1. HEAD 有效且指向存在的引用
2. 所有分支引用指向有效的 commit 对象
3. 每个 commit 的 tree 及其所有条目递归可达
4. 所有对象哈希值与内容一致
5. 检测悬空（不可达）对象

```sh
$ msync fsck
--- HEAD ---
--- Refs ---
--- Object Store ---
--- Reachability ---
--- Summary ---
Objects checked: 12
Errors:          0
Warnings:        0
Repository is clean.

$ msync fsck -v     # 详细模式：显示每个被检查的对象
```

### 垃圾回收（`msync gc`）

随着提交积累和分支删除，不可达对象（孤立提交、blob、tree）会使仓库膨胀。`msync gc` 识别并可选择性地删除它们。

```sh
$ msync gc
  Reachable objects: 12
  Total objects:     15
  Unreachable:       3
  Recoverable space: 1024 bytes (1.0 KB)

  3 unreachable objects found.
  Run 'msync gc --prune' to remove them.

$ msync gc --prune
  Pruned 3 objects, freed 1.0 KB.

$ msync gc
  Repository is clean, no garbage found.
```

## 许可证

Minus Sync (msync) 采用 **GNU Affero General Public License v3.0**（或任何更新版本）许可。完整文本见 [LICENSE](LICENSE)。

AGPLv3 要点：

- 你可以自由使用、修改和分发本软件。
- 如果你修改了软件并以网络服务形式运行，必须向该服务的用户提供修改后的源代码。
- 所有衍生作品也必须以 AGPLv3 许可。
