# Minus Sync (msync)  

A lightweight version control system written in C, inspired by Git but streamlined.

[简体中文](README_CN.md)

A lightweight version control system written in C, inspired by Git but streamlined. Runs on **Linux**, **Windows** (MinGW/MSVC), and **Android Termux**. Supports both client and server in a single binary (~70KB).

## Why msync?

| Feature | msync | Git |
|---------|-------|-----|
| Staging area | **No** — direct commit | Yes (`git add`) |
| Commit metadata | author, email, **hostname** | author, email |
| Object hashing | SHA-256 | SHA-1 (migrating to SHA-256) |
| Transport | Built-in TCP (default `:65530`) | HTTP / SSH / Git protocol |
| Server | Built-in (`msync serve`) | Separate daemon (`git daemon`) |
| Binary size | ~70 KB | ~10+ MB |
| Remotes per repo | Multiple, named | Multiple, named |
| Mirror daemon | Built-in (`msync mirror start`) | External tooling needed |
| Graph log | Built-in (`msync log --graphic`) | `git log --graph` |
| Ignore file | `.msyncign` (glob patterns) | `.gitignore` |
| Integrity check | Built-in (`msync fsck`) | `git fsck` |
| Garbage collection | Built-in (`msync gc`) | `git gc` |

## Building

```sh
# Requirements: GCC or Clang, GNU Make
make

# Optional: install to system
sudo make install PREFIX=/usr/local

# Windows (MinGW-w64)
make CC=gcc
# Output: msync.exe
```

No external dependencies beyond the C standard library and OS sockets.

## Quick Start

```sh
# 1. Initialize a repository
msync init

# 2. Configure identity
msync config user.name  "Your Name"
msync config user.email "you@example.com"

# 3. Create files and check status
echo "hello" > README.md
msync status
# On branch master
# New files:
#   new:  README.md

# 4. Commit directly (no staging)
msync commit -m "Initial commit"

# 5. View history
msync log
msync log --oneline        # compact
msync log --graphic         # branch graph
```

## Command Reference

### Repository Setup

```sh
msync init                              # Initialize a new repository
msync config <key>                      # Read a config value
msync config <key> <value>             # Set a config value
```

### Working with Changes

```sh
msync status                            # Show new, modified, deleted files
msync commit -m <message>              # Commit all changes (no staging area)
msync log [-n <count>]                 # Show commit history
msync log --oneline [-n <count>]       # One commit per line
msync log --graphic [-n <count>]        # Graphical branch history
```

### Branch Management

```sh
msync branch                            # List all branches
msync branch <name>                     # Create a new branch
msync branch -d <name>                  # Delete a branch
msync checkout <branch>                 # Switch to a branch
msync checkout <commit-hash>            # Detached HEAD at a commit
```

### Remote Repositories

```sh
msync remote add <name> <url>           # Add a named remote
msync remote list                       # List all remotes
msync remote remove <name>              # Remove a remote
```

### Syncing with Remotes

```sh
msync clone <url> [directory]           # Clone a remote repository
msync clone --name <name> <url> [dir]   # Clone with custom remote name
msync push   <remote|url> [branch]      # Push to remote (rejects non-fast-forward)
msync update <remote|url> [branch]      # Pull from remote (auto-merge if possible)
```

### Server

```sh
msync serve [-p <port>]                 # Start server (default port 65530)
```

### Ignoring Files

```sh
msync ignore add <pattern>              # Add an ignore pattern
msync ignore list                       # List all ignore patterns
msync ignore remove <pattern>           # Remove an ignore pattern
```

Patterns stored in `.msyncign`. See [.msyncign](#msyncign) for syntax.

### Repository Maintenance

```sh
msync fsck [-v]                         # Verify repository integrity
msync gc                                # Show unreachable objects
msync gc --prune                        # Remove unreachable objects
```

### Mirror (Repository Replication)

```sh
msync mirror once <source-url>          # One-shot full mirror
msync mirror start <url> [-i <sec>] [--serve <port>]
                                        # Continuous mirror daemon

# Configure via config for daemon mode
msync config mirror.source   host:65530
msync config mirror.interval 300
msync config mirror.serve-port 65530
msync mirror start                      # Uses config values
```

## Remote URL Format

```
host:port                    # e.g. 192.168.1.100:65530
msync://host:port            # protocol prefix (optional)
localhost:65530              # default port can be omitted → localhost (uses 65530)
```

If port is omitted, the default `65530` is used.

## Commit Record

Each commit stores:

```
tree <sha256-hash>
parent <sha256-hash>        (for merge commits: two parent lines)
author <name> <email> <unix-timestamp>
hostname <machine-hostname>

<message>
```

The `hostname` field enables distinguishing which machine a commit was made from — useful when the same author uses multiple devices.

## Repository Layout

```
.msync/
├── HEAD                    # Current branch (e.g. "ref: refs/heads/master")
├── config                  # INI-format configuration
├── index                   # Binary file index for fast status checks
├── objects/                # Content-addressable SHA-256 object store
│   └── XX/
│       └── XXXX...         # Objects stored by hash prefix
└── refs/
    ├── heads/              # Local branches
    │   ├── master
    │   └── feature-x
    └── remotes/            # Remote tracking refs
        ├── origin/
        │   └── master
        └── mirror/
            └── heads/
                └── master
```

### Object Types

| Type | Format | Purpose |
|------|--------|---------|
| blob | `blob <size>\0<content>` | File contents |
| tree | `tree <size>\0<mode> <name>\0<hash>...` | Directory listings |
| commit | `commit <size>\0<metadata>\n\n<message>` | Snapshots |

### Index Format

Binary file starting with magic `MSYN`, followed by version and entry count, then per-file records: path, SHA-256 hash, size, mtime, mode. Enables O(1) change detection via size/mtime comparison with hash verification as fallback.

## Network Protocol

Simple text-command / binary-data protocol over TCP:

```
Client                              Server
  │                                   │
  ├── LIST ──────────────────────────►│  List all refs
  │◄─ <count>                        │
  │◄─ <hash> <refname>               │  (per ref)
  │── OK ───────────────────────────►│
  │                                   │
  ├── FETCH ─────────────────────────►│  Fetch objects
  ├── WANT <hash>                     │
  ├── HAVE <hash>                     │  (optional, for incremental)
  ├── DONE ──────────────────────────►│
  │◄─ PACK <count>                   │  Binary pack follows
  │◄─ [hash][type][size][data]       │  (per object, 32+1+4+N bytes)
  │── OK ───────────────────────────►│
  │                                   │
  ├── PUSH ──────────────────────────►│  Push objects
  ├── UPDATE <ref> <old> <new> ──────►│  Server validates old hash
  │◄─ WANT_PACK / ERR ───────────────│  Non-fast-forward → ERR
  ├── PACK <count>                    │
  ├── [hash][type][size][data]        │
  │◄─ OK ────────────────────────────│
```

## Configuration Reference

```ini
[user]
    name = Your Name
    email = you@example.com

[remote "origin"]
    url = server.example.com:65530

[remote "backup"]
    url = backup.example.com:65530

[mirror]
    source = primary-server:65530
    interval = 300
    serve-port = 65530
```

## Workflow Examples

### Single-user local work

```sh
msync init
msync config user.name "Alice"
echo "project start" > main.c
msync commit -m "Initial"
# work, work...
msync commit -m "Add feature X"
msync log --oneline
```

### Team collaboration with central server

```sh
# Alice: start the server
alice$ msync serve -p 65530

# Bob: clone and work
bob$ msync clone alice-server:65530 project
bob$ cd project
bob$ msync config user.name "Bob"
bob$ echo "bob's code" >> main.c
bob$ msync commit -m "Bob's feature"
bob$ msync push origin

# Alice: pull Bob's changes
alice$ msync update origin
```

### Branch workflow

```sh
msync checkout -b feature-x
# work on feature...
msync commit -m "Feature X WIP"
msync checkout master
# fix critical bug...
msync commit -m "Hotfix"
msync checkout feature-x
msync update origin            # integrate master's hotfix
msync commit -m "Feature X done"
msync checkout master
# merge (via update from feature-x's remote, or manual)
```

### Mirror for backup / distributed teams

```sh
# Set up mirror on backup server
backup$ msync init
backup$ msync mirror start primary-server:65530 -i 300 --serve 65530

# Mirror syncs all branches every 5 minutes and serves read-only copies
# Team members in the backup region clone from the mirror
dev$ msync clone backup-server:65530 project
```

## Conflict Handling

### Push conflicts (non-fast-forward)

When the remote has advanced since your last sync, push is rejected:

```sh
$ msync push origin
Push rejected: the remote has newer commits.
Run 'msync update origin' first to integrate remote changes.
```

The server validates that your `old_hash` matches its current ref before accepting.

### Update merges

- **Fast-forward**: Remote is a direct descendant of local — automatic update.
- **Divergent**: Local and remote have diverged — msync finds the common ancestor and creates a merge commit with both histories preserved.

## .msyncign

The `.msyncign` file (compatible with `.gitignore` syntax) specifies intentionally untracked files.

### Pattern Syntax

| Syntax | Description | Example |
|--------|-------------|---------|
| `*.ext` | Wildcard matching | `*.o` matches `main.o`, `src/util.o` |
| `dir/` | Directory match | `build/` ignores the entire `build/` directory |
| `path/*.ext` | Path-scoped match | `src/*.o` matches `src/main.o` but not `lib/src/a.o` |
| `!pattern` | Negation (un-ignore) | `!important.o` excludes `important.o` from a wildcard rule |
| `# comment` | Comment line | `# build artifacts` |
| `**` | Recursive wildcard | `**/temp` matches `temp`, `a/temp`, `a/b/temp` |

### Example

```
# Build artifacts
*.o
*.exe
build/

# Dependencies
node_modules/

# Environment
.env
*.log

# But keep this one
!important.log
```

### Management Commands

```sh
msync ignore add "*.o"          # Add a pattern
msync ignore list               # Show all patterns
msync ignore remove "*.o"      # Remove a pattern
```

## Repository Maintenance

### Integrity Check (`msync fsck`)

Verifies the repository is not corrupted by checking:

1. HEAD is valid and points to an existing ref
2. All branch refs point to valid commit objects
3. Every commit's tree and all tree entries recursively exist
4. All object hashes match their content
5. Detects dangling (unreachable) objects

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

$ msync fsck -v     # Verbose: show each object as it is checked
```

### Garbage Collection (`msync gc`)

As commits accumulate and branches are deleted, unreferenced objects (orphaned commits, blobs, trees) can bloat the repository. `msync gc` identifies and optionally removes them.

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

## License

Minus Sync (msync) is licensed under the **GNU Affero General Public License v3.0** or (at your option) any later version. See [LICENSE](LICENSE) for the full text.

Key points of AGPLv3:

- You may use, modify, and distribute this software freely.
- If you modify the software and run it as a network service, you must make the modified source code available to users of that service.
- All derivative works must also be licensed under AGPLv3.
