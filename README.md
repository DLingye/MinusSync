# MinusSync (msync)

A version control and file synchronization system similar to git and lix, written in Go.

## Features

- **Version Control**: Full VCS with commits, branches, tags, merges, diffs, and logs
- **Remote Sync**: Push, pull, fetch, and clone over TCP/TLS
- **Binary Delta Sync**: Content-defined chunking (FastCDC) for efficient binary file transfers
- **Semantic Search**: Code-aware full-text search across tracked files
- **Cross-Platform**: Runs on Linux, Windows, and Android (Termux) — pure Go, CGO_ENABLED=0
- **Client & Server**: Built-in `msyncd` daemon with multi-repo hosting and systemd support

## Quick Start

### Install

```bash
# From source
git clone https://github.com/MinusSync/MinusSync.git
cd MinusSync
make build
sudo make install
```

### Basic Usage

```bash
# Initialize a repository
msync init my-project
cd my-project

# Add and commit files
echo "hello world" > README.md
msync add README.md
msync commit -m "Initial commit"

# View history
msync log --oneline

# Create a branch
msync branch feature-x
msync checkout feature-x

# Make changes and commit
echo "new feature" >> main.go
msync add -A
msync commit -m "Add feature X"

# Merge back
msync checkout main
msync merge feature-x

# Add a remote and push
msync remote add origin myserver.com:65530
msync push origin main

# Search tracked files
msync search "function parse"
```

### Server Setup

```bash
# Create config
sudo mkdir -p /etc/msyncd
sudo cp contrib/systemd/msyncd@.service /usr/lib/systemd/system/

# Start server
sudo systemctl enable --now msyncd@default
```

## Platform Support

| Platform | Status |
|----------|--------|
| Linux (amd64) | ✅ Full support |
| Linux (arm64) | ✅ Full support (Termux/Android) |
| Windows (amd64) | ✅ Full support |

## Building

```bash
# Current platform
make build

# Cross-compile all targets
make build-all

# Run tests
make test
```

## Commands

```
msync init        Initialize repository
msync clone       Clone remote repository
msync add         Stage files
msync commit      Create commit
msync status      Working tree status
msync log         Commit history
msync diff        Show changes
msync branch      Branch management
msync checkout    Switch branches
msync merge       Merge branches
msync tag         Tag management
msync remote      Remote management
msync push        Push to remote
msync pull        Pull from remote
msync fetch       Fetch from remote
msync search      Search tracked files
msync gc          Garbage collection
msync fsck        Integrity check
msync config      Get/set configuration
msync serve       Start embedded server
msync version     Print version
```

## License

AGPL-3.0-or-later
