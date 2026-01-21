---
name: sync
description: Sync git-tracked project files to the active sprite
argument-hint: "[--force]"
allowed-tools:
  - Bash
  - Read
---

# Sync Files to Sprite

Sync all git-tracked files from the current project to the active sprite's filesystem using the Sprites.dev Filesystem API.

## What Gets Synced

- All files tracked by git (output of `git ls-files`)
- Respects .gitignore automatically
- Skips files larger than 10MB
- Creates necessary directories on the sprite

## Behavior

1. Verify there's an active sprite with `sprite use`
2. Verify current directory is a git repository
3. Run the sync command: `$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sync <sprite-name> .`
4. Use `-v` flag for verbose output showing each file
5. Report results: files synced, any failures

## Command

```bash
# Basic sync
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sync <sprite-name> [source-dir]

# Verbose sync (shows each file)
$CLAUDE_PLUGIN_ROOT/bin/sprite-handoff sync -v <sprite-name> .
```

## Flags

- **--force**: Re-sync all files even if they may already exist

## Output

Report:
- Total files found
- Files successfully synced
- Files skipped (too large, binary)
- Any errors

## Tips

- First sync may take time for large projects
- Subsequent syncs can be faster with incremental approaches
- The sprite filesystem persists - files survive hibernation
