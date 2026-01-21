---
name: sessions
description: List, attach to, or kill running sprite sessions
argument-hint: "[list|attach|kill] [session-id]"
allowed-tools:
  - Bash
---

# Manage Sprite Sessions

Manage running sessions in the active sprite. Sessions are persistent command executions that can be detached and reattached.

## Actions

### List Sessions (default)
Show all running sessions:
```bash
sprite sessions list
```

### Attach to Session
Reconnect to a detached session:
```bash
sprite sessions attach <session-id>
```

### Kill Session
Terminate a running session:
```bash
sprite sessions kill <session-id>
```

## Behavior

1. Get the active sprite with `sprite use`
2. If no active sprite, inform user to run `/sprite:create` first
3. Parse the action from arguments (default: list)
4. Run the appropriate `sprite sessions` command
5. Display results

## Session Lifecycle

1. Start with `sprite exec -detachable "command"`
2. Detach with Ctrl+\ (keeps running in background)
3. Reattach with `sprite sessions attach <id>`
4. Kill with `sprite sessions kill <id>` when done

## Output

For list:
- Session ID
- Command being run
- Status (running, stopped)
- Duration

For attach:
- Reconnect to session output stream
- Note: Ctrl+\ to detach again

For kill:
- Confirmation of termination
