package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sprites-dev/sprite-handoff/internal/api"
	"github.com/sprites-dev/sprite-handoff/internal/config"
	"github.com/sprites-dev/sprite-handoff/internal/session"
	"github.com/sprites-dev/sprite-handoff/internal/sync"
	"github.com/sprites-dev/sprite-handoff/internal/worktree"
)

// handleSpriteRun syncs files and runs Claude on a sprite.
func handleSpriteRun(ctx context.Context, req *mcp.CallToolRequest, input SpriteRunInput) (*mcp.CallToolResult, SpriteRunOutput, error) {
	output := SpriteRunOutput{}

	// Get Claude token for remote authentication
	claudeToken, err := config.GetClaudeToken()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get Claude token: %w", err)
	}

	// Get sprite token for API calls
	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)
	sprite := client.Sprite(input.SpriteName)

	// Initialize session store
	store, err := session.NewStore()
	if err != nil {
		return nil, output, fmt.Errorf("failed to initialize session store: %w", err)
	}

	// Determine source directory
	sourceDir := input.SourceDir
	if sourceDir == "" {
		sourceDir = "."
	}

	var sess *session.Session
	var targetDir string

	if input.ResumeID != "" {
		// Resume existing session
		sess, err = store.Get(input.ResumeID)
		if err != nil {
			return nil, output, fmt.Errorf("failed to find session: %w", err)
		}

		// Verify worktree still exists
		wtMgr := worktree.NewManager(sprite)
		exists, err := wtMgr.WorktreeExists(ctx, sess)
		if err != nil {
			return nil, output, fmt.Errorf("failed to check worktree: %w", err)
		}
		if !exists {
			return nil, output, fmt.Errorf("worktree no longer exists: %s", sess.WorktreePath())
		}

		// Update last accessed
		sess.Touch()
		if err := store.Save(sess); err != nil {
			// Non-fatal, continue
		}

		targetDir = sess.WorktreePath()
		output.FilesSynced = 0 // Not syncing on resume
	} else {
		// New session - sync files to worktree
		absSourceDir := sourceDir
		if abs, err := filepath.Abs(sourceDir); err == nil && sourceDir == "." {
			absSourceDir = abs
		}
		projectName := getProjectName(absSourceDir)

		sess = session.NewSession(input.SpriteName, projectName, input.Prompt)

		wtMgr := worktree.NewManager(sprite)

		// Initialize bare repo if needed
		if err := wtMgr.InitRepo(ctx, projectName); err != nil {
			return nil, output, fmt.Errorf("failed to initialize repo: %w", err)
		}

		// Create worktree
		if err := wtMgr.CreateWorktree(ctx, sess); err != nil {
			return nil, output, fmt.Errorf("failed to create worktree: %w", err)
		}

		// Sync files to worktree
		syncer := sync.NewSyncer(client, input.SpriteName, sourceDir)
		result, err := syncer.SyncToWorktree(ctx, sess)
		if err != nil {
			return nil, output, fmt.Errorf("sync failed: %w", err)
		}

		output.FilesSynced = result.Synced

		// Create initial commit
		if err := wtMgr.Commit(ctx, sess, "Initial handoff sync"); err != nil {
			return nil, output, fmt.Errorf("failed to create initial commit: %w", err)
		}

		// Save session to local store
		if err := store.Save(sess); err != nil {
			// Non-fatal, continue
		}

		// Write session metadata to sprite
		if err := writeRemoteSessionMetadata(ctx, client, input.SpriteName, sess); err != nil {
			// Non-fatal, continue
		}

		targetDir = sess.WorktreePath()
	}

	// Build the claude command
	claudeCmd := "claude --print --dangerously-skip-permissions"
	if input.ResumeID != "" {
		claudeCmd += " --resume " + input.ResumeID
	} else {
		claudeCmd += " --session-id " + sess.ID
	}
	claudeCmd += " -p " + shellEscape(input.Prompt)
	if input.Model != "" {
		claudeCmd += fmt.Sprintf(" --model %s", input.Model)
	}
	if input.MaxBudget > 0 {
		claudeCmd += fmt.Sprintf(" --max-budget-usd %.2f", input.MaxBudget)
	}

	// Full command to run on sprite
	remoteCmd := fmt.Sprintf(
		"export CLAUDE_CODE_OAUTH_TOKEN=%q && cd %s && %s",
		claudeToken,
		targetDir,
		claudeCmd,
	)

	// Execute via SDK (non-TTY mode)
	spriteExec := sprite.CommandContext(ctx, "bash", "-c", remoteCmd)

	var stdout, stderr bytes.Buffer
	spriteExec.Stdout = &stdout
	spriteExec.Stderr = &stderr

	err = spriteExec.Run()

	output.SessionID = sess.ID
	output.WorktreePath = targetDir

	if err != nil {
		output.Status = "error"
		output.Message = fmt.Sprintf("Claude execution failed: %v\nStderr: %s", err, stderr.String())
	} else {
		output.Status = "completed"
		output.Message = stdout.String()
	}

	return nil, output, nil
}

// handleSpritePull pulls changed files from a session.
func handleSpritePull(ctx context.Context, req *mcp.CallToolRequest, input SpritePullInput) (*mcp.CallToolResult, SpritePullOutput, error) {
	output := SpritePullOutput{}

	// Load session
	store, err := session.NewStore()
	if err != nil {
		return nil, output, fmt.Errorf("failed to initialize session store: %w", err)
	}

	sess, err := store.Get(input.SessionID)
	if err != nil {
		return nil, output, fmt.Errorf("session not found: %w", err)
	}

	// Get sprite token
	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)
	sprite := client.Sprite(sess.Sprite)
	wtMgr := worktree.NewManager(sprite)

	// Get list of changed files
	changedFiles, err := wtMgr.ListChangedFiles(ctx, sess)
	if err != nil {
		return nil, output, fmt.Errorf("failed to list changed files: %w", err)
	}

	output.ChangedFiles = changedFiles

	if len(changedFiles) == 0 {
		output.FilesPulled = 0
		return nil, output, nil
	}

	if input.DryRun {
		// Just return the list of files
		return nil, output, nil
	}

	// Determine output directory
	var outputDir string
	if input.InPlace {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, output, fmt.Errorf("failed to get current directory: %w", err)
		}
		outputDir = cwd
	} else {
		repoRoot, err := getGitRepoRoot()
		if err != nil {
			return nil, output, fmt.Errorf("not in a git repository (use in_place=true to write directly): %w", err)
		}
		shortID := input.SessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		branch := fmt.Sprintf("sprite-handoff/%s", shortID)
		outputDir = fmt.Sprintf("%s/.sprite-worktrees/%s", repoRoot, shortID)
		output.WorktreePath = outputDir
		output.Branch = branch

		// Create local worktree
		if err := createLocalWorktree(repoRoot, outputDir, branch); err != nil {
			return nil, output, fmt.Errorf("failed to create local worktree: %w", err)
		}
	}

	output.OutputDir = outputDir

	// Pull each file
	for _, file := range changedFiles {
		// Read file from sprite
		content, err := client.ReadFileWithOptions(ctx, sess.Sprite, file, sess.WorktreePath())
		if err != nil {
			output.FilesFailed++
			continue
		}

		// Write to local filesystem
		localPath := filepath.Join(outputDir, file)

		// Create parent directory if needed
		dir := filepath.Dir(localPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				output.FilesFailed++
				continue
			}
		}

		if err := os.WriteFile(localPath, content, 0644); err != nil {
			output.FilesFailed++
			continue
		}

		output.FilesPulled++
	}

	// Commit changes if using worktree
	if !input.InPlace && output.FilesPulled > 0 {
		commitMsg := fmt.Sprintf("Pull from sprite-handoff session %s", input.SessionID[:8])
		commitWorktreeChanges(outputDir, commitMsg)
	}

	return nil, output, nil
}

// handleSessionsList lists all sessions.
func handleSessionsList(ctx context.Context, req *mcp.CallToolRequest, input SessionsListInput) (*mcp.CallToolResult, SessionsListOutput, error) {
	output := SessionsListOutput{}

	store, err := session.NewStore()
	if err != nil {
		return nil, output, fmt.Errorf("failed to initialize session store: %w", err)
	}

	sessions, err := store.List(input.SpriteName)
	if err != nil {
		return nil, output, fmt.Errorf("failed to list sessions: %w", err)
	}

	output.Sessions = make([]SessionInfo, len(sessions))
	for i, sess := range sessions {
		output.Sessions[i] = SessionInfo{
			ID:           sess.ID,
			Sprite:       sess.Sprite,
			Project:      sess.Project,
			Status:       sess.Status,
			Created:      sess.Created.Format("2006-01-02T15:04:05Z"),
			LastAccessed: sess.LastAccessed.Format("2006-01-02T15:04:05Z"),
			Prompt:       truncate(sess.Prompt, 100),
		}
	}
	output.Count = len(sessions)

	return nil, output, nil
}

// handleSessionsInfo gets detailed session information.
func handleSessionsInfo(ctx context.Context, req *mcp.CallToolRequest, input SessionsInfoInput) (*mcp.CallToolResult, SessionsInfoOutput, error) {
	output := SessionsInfoOutput{}

	store, err := session.NewStore()
	if err != nil {
		return nil, output, fmt.Errorf("failed to initialize session store: %w", err)
	}

	sess, err := store.Get(input.SessionID)
	if err != nil {
		return nil, output, fmt.Errorf("session not found: %w", err)
	}

	output.ID = sess.ID
	output.Sprite = sess.Sprite
	output.Project = sess.Project
	output.Branch = sess.Branch
	output.Worktree = sess.Worktree
	output.Status = sess.Status
	output.Created = sess.Created.Format("2006-01-02T15:04:05Z")
	output.LastAccessed = sess.LastAccessed.Format("2006-01-02T15:04:05Z")
	output.Prompt = sess.Prompt

	return nil, output, nil
}

// handleSessionsClean removes old sessions.
func handleSessionsClean(ctx context.Context, req *mcp.CallToolRequest, input SessionsCleanInput) (*mcp.CallToolResult, SessionsCleanOutput, error) {
	output := SessionsCleanOutput{}

	store, err := session.NewStore()
	if err != nil {
		return nil, output, fmt.Errorf("failed to initialize session store: %w", err)
	}

	var sessionsToClean []*session.Session

	if input.SessionID != "" {
		// Clean specific session
		sess, err := store.Get(input.SessionID)
		if err != nil {
			return nil, output, fmt.Errorf("session not found: %w", err)
		}
		sessionsToClean = append(sessionsToClean, sess)
	} else {
		// Get all sessions
		sessions, err := store.List(input.SpriteName)
		if err != nil {
			return nil, output, fmt.Errorf("failed to list sessions: %w", err)
		}
		sessionsToClean = sessions
	}

	// Get sprite token for API calls if cleaning remote
	var client *api.Client
	if input.Remote {
		spriteToken, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
		}
		client = api.NewClient(spriteToken)
	}

	for _, sess := range sessionsToClean {
		// Remove remote worktree if requested
		if input.Remote && client != nil {
			sprite := client.Sprite(sess.Sprite)
			wtMgr := worktree.NewManager(sprite)
			wtMgr.RemoveWorktree(ctx, sess) // Ignore errors
		}

		// Remove from local store
		if err := store.Delete(sess.ID); err != nil {
			output.Failed++
			continue
		}

		output.Cleaned++
		output.IDs = append(output.IDs, sess.ID)
	}

	return nil, output, nil
}

// handleSpriteSync syncs files without running Claude.
func handleSpriteSync(ctx context.Context, req *mcp.CallToolRequest, input SpriteSyncInput) (*mcp.CallToolResult, SpriteSyncOutput, error) {
	output := SpriteSyncOutput{}

	// Get sprite token
	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)

	sourceDir := input.SourceDir
	if sourceDir == "" {
		sourceDir = "."
	}

	opts := []sync.SyncerOption{}
	if input.BaseDir != "" {
		opts = append(opts, sync.WithBaseDir(input.BaseDir))
	}

	syncer := sync.NewSyncer(client, input.SpriteName, sourceDir, opts...)

	result, err := syncer.SyncGitFiles(ctx)
	if err != nil {
		return nil, output, fmt.Errorf("sync failed: %w", err)
	}

	output.HandoffID = result.HandoffID
	output.TargetDir = result.TargetDir
	output.Synced = result.Synced
	output.Skipped = result.Skipped
	output.Failed = result.Failed
	output.Total = result.TotalFiles

	return nil, output, nil
}

// handleSpriteRead reads a file from a sprite.
func handleSpriteRead(ctx context.Context, req *mcp.CallToolRequest, input SpriteReadInput) (*mcp.CallToolResult, SpriteReadOutput, error) {
	output := SpriteReadOutput{}

	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)
	content, err := client.ReadFile(ctx, input.SpriteName, input.Path)
	if err != nil {
		return nil, output, fmt.Errorf("failed to read file: %w", err)
	}

	output.Content = string(content)
	output.Path = input.Path

	return nil, output, nil
}

// handleSpriteWrite writes content to a file on a sprite.
func handleSpriteWrite(ctx context.Context, req *mcp.CallToolRequest, input SpriteWriteInput) (*mcp.CallToolResult, SpriteWriteOutput, error) {
	output := SpriteWriteOutput{}

	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)
	err = client.WriteFile(ctx, input.SpriteName, input.Path, []byte(input.Content))
	if err != nil {
		return nil, output, fmt.Errorf("failed to write file: %w", err)
	}

	output.Path = input.Path
	output.Success = true

	return nil, output, nil
}

// handleSpriteList lists files in a directory on a sprite.
func handleSpriteList(ctx context.Context, req *mcp.CallToolRequest, input SpriteListInput) (*mcp.CallToolResult, SpriteListOutput, error) {
	output := SpriteListOutput{}

	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	path := input.Path
	if path == "" {
		path = "."
	}

	client := api.NewClient(spriteToken)
	entries, err := client.ListDir(ctx, input.SpriteName, path)
	if err != nil {
		return nil, output, fmt.Errorf("failed to list directory: %w", err)
	}

	output.Entries = make([]FileEntry, len(entries))
	for i, entry := range entries {
		output.Entries[i] = FileEntry{
			Name:    entry.Name,
			Path:    entry.Path,
			Size:    entry.Size,
			IsDir:   entry.IsDir,
			Mode:    entry.Mode,
			ModTime: entry.ModTime,
		}
	}
	output.Path = path
	output.Count = len(entries)

	return nil, output, nil
}

// handleSpriteDelete deletes a file or directory from a sprite.
func handleSpriteDelete(ctx context.Context, req *mcp.CallToolRequest, input SpriteDeleteInput) (*mcp.CallToolResult, SpriteDeleteOutput, error) {
	output := SpriteDeleteOutput{}

	spriteToken, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		return nil, output, fmt.Errorf("failed to get sprite token: %w", err)
	}

	client := api.NewClient(spriteToken)
	err = client.Delete(ctx, input.SpriteName, input.Path, api.DeleteOptions{
		Recursive: input.Recursive,
	})
	if err != nil {
		return nil, output, fmt.Errorf("failed to delete: %w", err)
	}

	output.Path = input.Path
	output.Success = true

	return nil, output, nil
}

// Helper functions

// writeRemoteSessionMetadata writes session metadata to the sprite.
func writeRemoteSessionMetadata(ctx context.Context, client *api.Client, spriteName string, sess *session.Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}

	return client.WriteFileWithOptions(ctx, spriteName, sess.ID+".json", data, api.WriteOptions{
		WorkingDir: session.SessionsMetadataDir,
		MkdirAll:   true,
	})
}

// getProjectName extracts a project name from a directory path.
func getProjectName(dir string) string {
	name := dir
	if abs, err := os.Getwd(); err == nil && (dir == "." || dir == "") {
		name = abs
	}

	base := filepath.Base(name)
	if base == "" || base == "." || base == "/" {
		return "project"
	}
	return base
}

// shellEscape escapes a string for safe use in shell single quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getGitRepoRoot returns the root directory of the current git repository.
func getGitRepoRoot() (string, error) {
	// Use os/exec to run git command
	cmd := execCommand("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// createLocalWorktree creates a local git worktree for pulling changes.
func createLocalWorktree(repoRoot, worktreePath, branch string) error {
	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, check if it's valid
		checkCmd := execCommand("git", "-C", worktreePath, "rev-parse", "--git-dir")
		if err := checkCmd.Run(); err == nil {
			// Valid worktree, reuse it
			return nil
		}
		// Invalid, remove and recreate
		os.RemoveAll(worktreePath)
	}

	// Create parent directory
	dir := filepath.Dir(worktreePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create worktree directory: %w", err)
		}
	}

	// Create the worktree with a new branch based on current HEAD
	cmd := execCommand("git", "-C", repoRoot, "worktree", "add", "-b", branch, worktreePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Branch might already exist, try without -b
		cmd2 := execCommand("git", "-C", repoRoot, "worktree", "add", worktreePath, branch)
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("failed to create worktree: %s", stderr.String())
		}
	}

	return nil
}

// commitWorktreeChanges commits all changes in a worktree.
func commitWorktreeChanges(worktreePath, message string) error {
	// Stage all changes
	addCmd := execCommand("git", "-C", worktreePath, "add", "-A")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Commit (ignore error if nothing to commit)
	commitCmd := execCommand("git", "-C", worktreePath, "commit", "-m", message)
	commitCmd.Run() // Ignore error - might be "nothing to commit"

	return nil
}

// execCommand is a wrapper for exec.Command.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
