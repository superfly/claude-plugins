---
name: create
description: Create a new sprite or select an existing one for remote execution
argument-hint: "[sprite-name]"
allowed-tools:
  - Bash
---

# Create or Select a Sprite

Create a new Sprites.dev cloud sandbox or select an existing one as the active sprite for this project.

## Behavior

**If a sprite name is provided:**
1. Check if sprite exists with `sprite list`
2. If it exists, set it as active with `sprite use <name>`
3. If it doesn't exist, create it with `sprite create <name>` then set as active

**If no sprite name is provided:**
1. List existing sprites with `sprite list`
2. Show the list to the user
3. Ask if they want to use an existing sprite or create a new one

## Commands to Use

```bash
# List sprites
sprite list

# Create new sprite
sprite create <name>

# Set active sprite for current directory
sprite use <name>

# Check current active sprite
sprite use
```

## Output

After completing, report:
- Sprite name
- Whether it was created or already existed
- Confirm it's set as active

## Tips

- Sprite names should be lowercase, alphanumeric with hyphens
- Sprites auto-hibernate after 30 seconds of inactivity (no cost when idle)
- Sprites have persistent filesystems - data survives hibernation
