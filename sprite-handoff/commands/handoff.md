---
name: handoff
description: Hand off the current task to Claude Code running on a sprite
argument-hint: "<task-description>"
allowed-tools:
  - Bash
  - Read
  - Glob
---

# Hand Off to Remote Claude Code

Transfer the current work to a Claude Code instance running on a sprite in the cloud. This enables background processing, parallel work, or offloading long-running tasks.

## Workflow

1. **Ensure sprite is ready**
   - Check for active sprite with `sprite use`
   - If none, create one or ask user to run `/sprite:create`

2. **Generate handoff context**
   - Create a summary of the current conversation/task
   - Combine with user's task description
   - Format as a prompt for the remote Claude

3. **Run the handoff using the CLI tool**
   ```bash
   $CLAUDE_PLUGIN_ROOT/bin/sprite-handoff run <sprite-name> "<prompt>" [flags]
   ```

   The `run` command handles everything:
   - Creates git worktree for session isolation
   - Syncs all git-tracked files
   - Creates initial commit
   - Retrieves Claude OAuth token
   - Launches Claude Code on the sprite

   **Available flags:**
   - `--model <model>` - Specify model (sonnet, opus, haiku)
   - `--max-budget <amount>` - Set spending limit in USD
   - `--source <dir>` - Source directory (default: current dir)
   - `-v` - Verbose output

4. **Return session info**
   - The command outputs the session ID
   - Explain how to resume: `sprite-handoff run <sprite> "<prompt>" --resume <session-id>`
   - Explain how to pull changes: `sprite-handoff pull <session-id>`

## Handoff Prompt Format

The prompt passed to the `run` command should include context:

```
## Context
[Summary of conversation and what was being worked on]

## Project State
[Key files, recent changes, current status]

## Task
[User's task description - what to continue working on]

## IMPORTANT: Commit Your Changes
You MUST commit all changes before finishing. The user will pull your work
using `git diff`, so uncommitted changes will be lost.

When done, run:
  git add -A && git commit -m "Description of changes"

## Instructions
- Work autonomously on this task
- Commit all changes with a descriptive message when complete
- Write a summary of what was done to HANDOFF_RESULT.md
```

**Why commits matter:** The `sprite-handoff pull` command uses `git diff` to find changed files. If changes aren't committed, they won't be pulled back to the user's machine.

## Arguments

The user should provide a task description explaining what the remote Claude should work on:
- "Run the full test suite and fix any failures"
- "Implement the remaining API endpoints"
- "Refactor the database layer for better performance"

## Output

After handoff, report:
```
=== Handoff Complete ===
Sprite: my-sprite
Session ID: <uuid>
Worktree: /handoffs/my-project.worktrees/<uuid>

Remote Claude is now working on:
  "<task description>"

To resume this session:
  sprite-handoff run my-sprite "<prompt>" --resume <session-id>

To pull changes when complete:
  sprite-handoff pull <session-id>

To list all sessions:
  sprite-handoff sessions list
```

## Session Management

After handoff, the user can:

```bash
# Resume the session with a new prompt
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff run <sprite> "Continue" --resume <session-id>

# Pull changed files back to local machine
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff pull <session-id>
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff pull <session-id> --dry-run  # Preview only

# List all sessions
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sessions list
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sessions list <sprite-name>

# Get session details
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sessions info <session-id>

# Clean up old sessions
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sessions clean --session <id>
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sessions clean --remote  # Also remove worktrees
```

## Tips

- Each session gets its own git worktree - no interference between sessions
- Resume sessions by ID - no need to re-sync files
- The remote Claude runs independently - you can close your terminal
- Pull changes back when the remote work is complete
- Sprites hibernate when idle (no cost) and wake instantly
- The Claude token is passed via `CLAUDE_CODE_OAUTH_TOKEN` environment variable
