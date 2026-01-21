# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sprite-handoff is a Claude Code plugin for offloading sessions to Sprites.dev cloud sandboxes. It provides file sync capabilities, git worktree-based session isolation, and session management for remote execution.

## Build Commands

```bash
make build          # Build for current platform (output: bin/sprite-handoff)
make build-all      # Build for darwin/linux amd64/arm64
make test           # Run unit tests
make test-unit      # Run unit tests only (./internal/...)
make test-coverage  # Generate coverage.html report
make lint           # Run golangci-lint
make fmt            # Format code with gofmt
make vet            # Run go vet

# Run a single test
go test -v -run TestName ./internal/sync/...
```

### Integration/E2E Tests

Require a real sprite and environment variables:

```bash
export SPRITE_NAME=my-test-sprite
make test-integration
make test-e2e
make test-all       # Runs everything including integration/E2E
```

## Architecture

### Go CLI (`cmd/sprite-handoff/main.go`)

Cobra-based CLI with these subcommands:

**Core commands:**
- `run <sprite-name> <prompt>` - Sync files and run Claude Code on a sprite (main handoff command)
- `pull <session-id>` - Pull changed files from a session back to local
- `sessions list [sprite]` - List all handoff sessions
- `sessions info <id>` - Show session details
- `sessions clean` - Remove old sessions

**File operations:**
- `sync <sprite-name> [source-dir]` - Sync git-tracked files to sprite (legacy, prefer `run`)
- `read <sprite-name> <path>` - Read file from sprite filesystem
- `write <sprite-name> <remote-path> <local-path>` - Write file to sprite
- `list <sprite-name> [path]` - List directory contents
- `delete <sprite-name> <path>` - Delete file/directory

**Auth:**
- `token` - Print sprite auth token
- `claude-token` - Print Claude Code OAuth token (for remote instances)

### Internal Packages

- **`internal/api`** - Sprites.dev Filesystem API client. Handles read/write/list/delete operations via HTTP. Default base URL: `https://api.sprites.dev/v1`. Files sync to `/handoffs/` by default.

- **`internal/session`** - Session metadata management. `Session` struct tracks ID, sprite, project, worktree path, branch, timestamps, status. `Store` manages local JSON cache at `~/.sprite-handoff/sessions.json`.

- **`internal/worktree`** - Git worktree operations on remote sprites via `sprite exec`. Handles repo initialization, worktree creation/removal, commits, and listing changed files.

- **`internal/sync`** - Git-aware file syncer. Uses `git ls-files` to determine which files to sync. Supports both legacy handoff directories and session worktrees. Max file size: 10MB.

- **`internal/config`** - Authentication token management. Platform-specific keychain support:
  - **macOS (`keychain_darwin.go`)**: Reads sprite token from keychain using `security` CLI. Also retrieves Claude Code OAuth token from "Claude Code-credentials" keychain entry.
  - **Other platforms (`keychain_other.go`)**: Falls back to env vars (`SPRITE_TOKEN`, `ANTHROPIC_API_KEY`) and config file (`~/.sprites/sprites.json`).

### Plugin Structure

- **`commands/`** - Claude Code slash commands (`/sprite:handoff`, `/sprite:create`, `/sprite:sync`, etc.)
- **`agents/sprite-orchestrator.md`** - Agent for coordinating full handoff workflows using the `run` command
- **`skills/`** - Plugin skills

## Key Concepts

**Session**: Each handoff creates a session with a UUID. Sessions track the sprite, project, worktree path, and status. Sessions are stored locally at `~/.sprite-handoff/sessions.json` and remotely at `/handoffs/.sessions/<id>.json`.

**Git Worktree**: Each session gets its own git worktree for isolation. Directory structure on sprite:
```
/handoffs/
├── my-project/                    # Bare git repo (one per project)
├── my-project.worktrees/
│   └── <session-uuid>/            # Working directory for this session
└── .sessions/
    └── <session-uuid>.json        # Session metadata
```

**Token Priority**: Both sprite and Claude tokens check: (1) environment variable, (2) config file, (3) macOS keychain.

**Remote Claude Execution**: The `run` command retrieves local OAuth credentials and passes them to Claude Code on sprites via `CLAUDE_CODE_OAUTH_TOKEN` env var.

## Common Workflows

**New handoff:**
```bash
./bin/sprite-handoff run my-sprite "Run tests and fix failures"
```

**Resume session:**
```bash
./bin/sprite-handoff run my-sprite "Continue" --resume <session-id>
```

**Pull changes back:**
```bash
./bin/sprite-handoff pull <session-id>
```

**List sessions:**
```bash
./bin/sprite-handoff sessions list
```
