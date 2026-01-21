// Package mcpserver provides an MCP server for sprite-handoff operations.
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is set at build time via ldflags.
var Version = "0.1.0"

// NewServer creates a new MCP server with all sprite-handoff tools registered.
func NewServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "sprite-handoff",
			Version: Version,
		},
		nil,
	)

	// Core Workflow tools
	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_run",
		Description: `Sync files to a sprite and run Claude Code with a prompt.

This is the main handoff tool. It:
1. Creates a git worktree on the sprite for session isolation
2. Syncs all git-tracked files to the worktree
3. Launches Claude Code on the sprite with your prompt
4. Returns a session ID for resuming or pulling changes later

Use resume_id to continue a previous session instead of starting fresh.`,
	}, handleSpriteRun)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_pull",
		Description: `Pull changed files from a sprite session back to the local machine.

Downloads files that were modified during the handoff session. By default,
creates a local git worktree to isolate the changes for review. Use in_place
to write directly to the current directory instead.`,
	}, handleSpritePull)

	// Session Management tools
	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_sessions_list",
		Description: `List all handoff sessions.

Returns a list of sessions with their IDs, sprites, projects, status, and timestamps.
Optionally filter by sprite name.`,
	}, handleSessionsList)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_sessions_info",
		Description: `Get detailed information about a specific session.

Returns full session details including worktree path, branch, and original prompt.`,
	}, handleSessionsInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_sessions_clean",
		Description: `Remove old or abandoned sessions.

Cleans up sessions from the local cache. Use remote=true to also remove
the worktrees from the sprites.`,
	}, handleSessionsClean)

	// File Operations tools
	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_sync",
		Description: `Sync git-tracked files to a sprite without running Claude.

Useful for preparing files on a sprite before manual operations.
Files are synced to /handoffs/<dirname>-HHMMSS/ by default.`,
	}, handleSpriteSync)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_read",
		Description: `Read a file from a sprite filesystem.

Returns the content of a file at the specified path on the sprite.`,
	}, handleSpriteRead)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_write",
		Description: `Write content to a file on a sprite filesystem.

Creates or overwrites a file at the specified path with the given content.`,
	}, handleSpriteWrite)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_list",
		Description: `List files and directories on a sprite filesystem.

Returns directory entries with names, sizes, permissions, and types.`,
	}, handleSpriteList)

	mcp.AddTool(server, &mcp.Tool{
		Name: "sprite_delete",
		Description: `Delete a file or directory from a sprite filesystem.

Use recursive=true to delete directories with contents.`,
	}, handleSpriteDelete)

	return server
}

// Run starts the MCP server over stdio transport.
func Run(ctx context.Context) error {
	server := NewServer()
	return server.Run(ctx, &mcp.StdioTransport{})
}
