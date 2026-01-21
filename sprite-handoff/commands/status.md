---
name: status
description: Show current sprite status, disk usage, and running sessions
allowed-tools:
  - Bash
---

# Sprite Status

Display comprehensive status information for the active sprite including disk usage, running sessions, and general health.

## Information Gathered

1. **Active Sprite**: Current sprite name for this directory
2. **Sprite List**: All available sprites
3. **Running Sessions**: Active sessions in the sprite
4. **Disk Usage**: Filesystem usage in the sprite
5. **Running Processes**: What's currently executing

## Commands to Run

```bash
# Get active sprite
sprite use

# List all sprites (shows status)
sprite list

# List sessions
sprite sessions list

# Check disk usage (run in sprite)
sprite exec "df -h /home/sprite"

# Check running processes
sprite exec "ps aux"
```

## Behavior

1. Get the active sprite name
2. If no active sprite, just list available sprites
3. If there is an active sprite, gather all status info
4. Present a summary to the user

## Output Format

```
=== Sprite Status ===
Active Sprite: my-dev-env
Status: Running / Hibernated

=== Disk Usage ===
Used: 2.1G / 10G (21%)

=== Running Sessions ===
ID      Command              Duration
abc123  npm run dev          2h 15m
def456  pytest --watch       45m

=== Processes ===
(output of ps aux)
```

## Tips

- Sprites auto-hibernate after 30s of inactivity
- They wake instantly on any command
- No compute charges while hibernated
