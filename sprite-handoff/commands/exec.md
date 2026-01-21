---
name: exec
description: Execute commands in the active sprite cloud sandbox
argument-hint: "<command> [--detach] [--env KEY=VAL]"
allowed-tools:
  - Bash
---

# Execute Commands in Sprite

Run commands in the active sprite cloud sandbox. Supports one-off execution, detachable sessions for background tasks, and environment variables.

## Execution Modes

### One-off (default)
Run a command and return the output:
```bash
sprite exec "npm run build"
```

### Detachable Session
Run in background, can detach and reattach later:
```bash
sprite exec -detachable "npm run dev"
# Press Ctrl+\ to detach
```

### With Environment Variables
```bash
sprite exec -env NODE_ENV=production,DEBUG=true "npm start"
```

### With Working Directory
```bash
sprite exec -dir /home/sprite/myproject "npm test"
```

## Argument Parsing

Parse the user's input to determine:
- **Command**: The shell command to run (required)
- **--detach or -d**: Use detachable mode for long-running tasks
- **--env or -e**: Environment variables (KEY=VAL,KEY2=VAL2)
- **--dir**: Working directory in the sprite

## Behavior

1. Get the active sprite name with `sprite use`
2. If no active sprite, tell user to run `/sprite:create` first
3. Build the appropriate `sprite exec` command with flags
4. Run the command and show output
5. For detachable mode, explain how to detach (Ctrl+\) and reattach

## Tips

- Use detachable mode for dev servers, builds, or anything long-running
- Sessions persist even if you close your terminal
- Use `/sprite:sessions` to manage running sessions
