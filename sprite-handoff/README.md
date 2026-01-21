# sprite-handoff

A Claude Code plugin for offloading sessions to [Sprites.dev](https://sprites.dev) cloud sandboxes.

## Features

- **One-Command Handoff**: Sync files and launch Claude Code on a sprite with a single command
- **Git Worktree Isolation**: Each session gets its own git worktree for clean state management
- **Session Persistence**: Resume sessions by ID - no need to re-sync files
- **Pull Changes Back**: Fetch modified files from remote sessions to your local machine
- **Background Tasks**: Run long-running builds/tests in the cloud while you work locally

## Prerequisites

1. **Sprites CLI installed and authenticated**:
   ```bash
   # Install (if not already)
   curl -sSL https://sprites.dev/install.sh | bash

   # Authenticate
   sprite org auth
   ```

2. **Git repository**: The sync command uses git to determine which files to transfer

3. **Go 1.21+** (for building the CLI tool)

## Installation

Install via the Superfly Plugins marketplace:

```
/plugin marketplace add superfly/claude-plugins
/plugin install sprite-handoff@superfly-plugins
```

To manually check for updates:

```
/plugin marketplace update superfly-plugins
```

## Quick Start

```bash
# Hand off a task to a sprite (syncs files + launches Claude)
./bin/sprite-handoff run my-sprite "Run the test suite and fix any failures"

# Resume a previous session
./bin/sprite-handoff run my-sprite "Continue fixing the tests" --resume <session-id>

# Pull changed files back to local machine
./bin/sprite-handoff pull <session-id>

# List all sessions
./bin/sprite-handoff sessions list
```

## CLI Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `run <sprite> <prompt>` | Sync files and run Claude Code on a sprite |
| `pull <session-id>` | Pull changed files from a session back to local |
| `sessions list` | List all handoff sessions |
| `sessions info <id>` | Show details about a session |
| `sessions clean` | Remove old sessions |

### File Operations

| Command | Description |
|---------|-------------|
| `sync <sprite> [dir]` | Sync git-tracked files to a sprite |
| `read <sprite> <path>` | Read a file from sprite filesystem |
| `write <sprite> <remote> <local>` | Write a local file to sprite |
| `list <sprite> [path]` | List directory contents |
| `delete <sprite> <path>` | Delete a file or directory |

### Authentication

| Command | Description |
|---------|-------------|
| `token` | Print sprite auth token |
| `claude-token` | Print Claude Code OAuth token |

## The `run` Command

The main command for handing off work to a sprite:

```bash
sprite-handoff run <sprite-name> <prompt> [flags]

Flags:
  --resume, -r <id>     Resume an existing session
  --model <model>       Claude model (sonnet, opus, haiku)
  --max-budget <amt>    Maximum budget in USD
  --source <dir>        Source directory (default: .)
  -v, --verbose         Verbose output
```

### New Session Flow

1. Creates a bare git repo on the sprite (if first time for this project)
2. Creates a new git worktree for session isolation
3. Syncs all git-tracked files to the worktree
4. Creates an initial commit
5. Saves session metadata (locally and remotely)
6. Launches Claude Code with your prompt

### Resume Flow

1. Looks up session by ID from local cache
2. Verifies worktree still exists
3. Launches Claude Code with `--resume` flag
4. No re-sync needed - worktree preserves state

## Architecture

### Directory Structure on Sprite

```
/handoffs/
├── my-project/                    # Bare git repo (one per project)
├── my-project.worktrees/
│   ├── <session-uuid-1>/          # Working directory for session 1
│   └── <session-uuid-2>/          # Working directory for session 2
└── .sessions/
    ├── <session-uuid-1>.json      # Session metadata
    └── <session-uuid-2>.json
```

### Local Session Cache

Sessions are cached locally at `~/.sprite-handoff/sessions.json` for quick lookups.

## Using as a Claude Code Plugin

When installed as a Claude Code plugin, there are three ways to interact with sprite-handoff:

### 1. Slash Commands (Primary Entry Point)

| Command | Description |
|---------|-------------|
| `/sprite:handoff "prompt"` | **Main entry** - hand off current task to a remote sprite |
| `/sprite:create` | Create a new sprite or select an existing one |
| `/sprite:sync` | Sync git-tracked files to the active sprite |
| `/sprite:exec` | Execute commands in the active sprite |
| `/sprite:sessions` | List, attach, or kill running sessions |
| `/sprite:status` | Show current sprite status, disk usage, and sessions |

### 2. Natural Language via Agent

The `sprite-orchestrator` agent triggers automatically on phrases like:
- "Hand this off to a sprite"
- "Run this in a sprite"
- "Let the sprite handle this"
- "Offload this work to the cloud"

The agent coordinates the full workflow: sprite selection/creation, context generation, file sync, and remote Claude launch.

### 3. Direct CLI

The Go binary can also be invoked directly from your terminal:

```bash
./bin/sprite-handoff run my-sprite "Run tests"
./bin/sprite-handoff pull <session-id>
./bin/sprite-handoff sessions list
```

### Recommended Workflow

1. **Invoke**: Use `/sprite:handoff "Run tests and fix failures"` or natural language
2. **Automatic**: Plugin syncs files, creates isolated git worktree, launches remote Claude
3. **Continue**: You receive a session ID and can continue local work
4. **Retrieve**: Pull changes back with `/sprite:sessions` or `sprite-handoff pull <session-id>`

## Development

### Building

```bash
make build          # Build for current platform
make build-all      # Build for all platforms
```

### Testing

```bash
make test           # Run unit tests
make test-unit      # Run unit tests only
make test-coverage  # Run with coverage report
```

### Integration & E2E Tests

Integration and E2E tests require a real sprite:

```bash
# Set environment variables
export SPRITE_NAME=my-test-sprite

# Run integration tests
make test-integration

# Run E2E tests
make test-e2e

# Run all tests
make test-all
```

### Code Quality

```bash
make fmt            # Format code
make lint           # Run linter
make vet            # Run go vet
```

## Project Structure

```
sprite-handoff/
├── .claude-plugin/
│   └── plugin.json         # Plugin manifest
├── cmd/
│   └── sprite-handoff/
│       └── main.go         # CLI entry point
├── internal/
│   ├── api/                # Sprites Filesystem API client
│   ├── config/             # Token and config management
│   ├── session/            # Session metadata management
│   ├── sync/               # Git file sync logic
│   └── worktree/           # Git worktree operations
├── tests/
│   ├── integration/        # Integration tests
│   └── e2e/                # End-to-end tests
├── commands/               # Plugin commands
├── agents/                 # Plugin agents
├── bin/                    # Built binaries
├── Makefile
└── README.md
```

## Authentication

### Sprite Token

Used for Sprites.dev API calls. Read from:
1. `SPRITE_TOKEN` environment variable
2. macOS keychain (sprites CLI stores it there)
3. `~/.sprites/sprites.json` config file

### Claude Token

Used to authenticate Claude Code on remote sprites. Read from:
1. `ANTHROPIC_API_KEY` environment variable
2. macOS keychain ("Claude Code-credentials" entry)

The token is passed to remote Claude via `CLAUDE_CODE_OAUTH_TOKEN` environment variable.

## License

MIT
