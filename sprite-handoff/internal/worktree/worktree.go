// Package worktree manages git worktrees on remote sprites
package worktree

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	sprites "github.com/superfly/sprites-go"

	"github.com/sprites-dev/sprite-handoff/internal/session"
)

// Manager handles git worktree operations on a sprite
type Manager struct {
	sprite  *sprites.Sprite
	verbose bool
}

// NewManager creates a new worktree manager
func NewManager(sprite *sprites.Sprite) *Manager {
	return &Manager{
		sprite: sprite,
	}
}

// WithVerbose enables verbose output
func (m *Manager) WithVerbose(verbose bool) *Manager {
	m.verbose = verbose
	return m
}

// InitRepo initializes a bare git repo if it doesn't exist
func (m *Manager) InitRepo(ctx context.Context, project string) error {
	repoPath := fmt.Sprintf("%s/%s", session.HandoffsDir, project)

	// Check if repo already exists
	checkCmd := fmt.Sprintf("test -d %s/.git || test -f %s/HEAD", repoPath, repoPath)
	if err := m.execQuiet(ctx, checkCmd); err == nil {
		// Repo already exists
		return nil
	}

	// Create the bare repo
	initCmd := fmt.Sprintf("mkdir -p %s && git init --bare %s", repoPath, repoPath)
	if err := m.exec(ctx, initCmd); err != nil {
		return fmt.Errorf("failed to initialize repo: %w", err)
	}

	return nil
}

// CreateWorktree creates a new worktree for a session
func (m *Manager) CreateWorktree(ctx context.Context, sess *session.Session) error {
	// Ensure worktrees directory exists
	worktreesDir := sess.WorktreesDir()
	mkdirCmd := fmt.Sprintf("mkdir -p %s", worktreesDir)
	if err := m.exec(ctx, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	repoPath := sess.RepoPath()
	worktreePath := sess.WorktreePath()
	branch := sess.Branch

	// Check if the repo has any commits by trying to resolve HEAD to an actual commit.
	// Note: bare repos created with 'git init --bare' have HEAD pointing to refs/heads/master
	// but the branch doesn't exist until the first commit. In this state, 'git rev-parse HEAD'
	// returns the literal string "HEAD" with exit code 0, so we must use --verify HEAD^{commit}
	// which properly fails when HEAD doesn't point to a valid commit object.
	hasCommits := fmt.Sprintf("git -C %s rev-parse --verify HEAD^{commit} >/dev/null 2>&1", repoPath)
	if err := m.execQuiet(ctx, hasCommits); err != nil {
		// No commits yet - use --orphan to create worktree with unborn branch
		// This avoids the "invalid reference: HEAD" error on bare repos
		createCmd := fmt.Sprintf("git -C %s worktree add --orphan -b %s %s", repoPath, branch, worktreePath)
		if err := m.exec(ctx, createCmd); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		return nil
	}

	// Repo has commits - create worktree with a new branch from HEAD
	createCmd := fmt.Sprintf("git -C %s worktree add -b %s %s", repoPath, branch, worktreePath)
	if err := m.exec(ctx, createCmd); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

// WorktreeExists checks if a worktree exists
func (m *Manager) WorktreeExists(ctx context.Context, sess *session.Session) (bool, error) {
	checkCmd := fmt.Sprintf("test -d %s", sess.WorktreePath())
	if err := m.execQuiet(ctx, checkCmd); err != nil {
		return false, nil
	}
	return true, nil
}

// Commit creates a commit in the worktree
func (m *Manager) Commit(ctx context.Context, sess *session.Session, message string) error {
	worktreePath := sess.WorktreePath()

	// Configure git user and commit all changes
	commitCmd := fmt.Sprintf(`
		cd %s && \
		git config user.email "handoff@sprites.dev" && \
		git config user.name "Sprite Handoff" && \
		git add -A && \
		git commit -m %q || true
	`, worktreePath, message)

	if err := m.exec(ctx, commitCmd); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// ListChangedFiles returns files changed since initial commit
func (m *Manager) ListChangedFiles(ctx context.Context, sess *session.Session) ([]string, error) {
	worktreePath := sess.WorktreePath()

	// Get the initial commit (first commit on this branch)
	// We compare against HEAD~1 if it exists, otherwise list all tracked files
	// Note: Use -c safe.directory to handle ownership mismatch when exec runs as different user than Claude Code
	diffCmd := fmt.Sprintf(`
		cd %s && \
		if git -c safe.directory=%s rev-parse HEAD~1 >/dev/null 2>&1; then
			git -c safe.directory=%s diff --name-only HEAD~1 HEAD
			git -c safe.directory=%s diff --name-only HEAD
		else
			git -c safe.directory=%s ls-files
		fi
	`, worktreePath, worktreePath, worktreePath, worktreePath, worktreePath)

	output, err := m.execOutput(ctx, diffCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed files: %w", err)
	}

	// Parse output into file list
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			unique = append(unique, f)
		}
	}

	return unique, nil
}

// RemoveWorktree removes a worktree
func (m *Manager) RemoveWorktree(ctx context.Context, sess *session.Session) error {
	repoPath := sess.RepoPath()
	worktreePath := sess.WorktreePath()

	removeCmd := fmt.Sprintf("git -C %s worktree remove --force %s 2>/dev/null || rm -rf %s",
		repoPath, worktreePath, worktreePath)

	if err := m.exec(ctx, removeCmd); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// exec runs a command on the sprite using the SDK
func (m *Manager) exec(ctx context.Context, cmd string) error {
	spriteCmd := m.sprite.CommandContext(ctx, "bash", "-c", cmd)

	var stderr bytes.Buffer
	spriteCmd.Stderr = &stderr

	if m.verbose {
		spriteCmd.Stdout = nil // Let it print to stdout
	}

	if err := spriteCmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}

	return nil
}

// execQuiet runs a command without output, just checking exit code
func (m *Manager) execQuiet(ctx context.Context, cmd string) error {
	spriteCmd := m.sprite.CommandContext(ctx, "bash", "-c", cmd)
	return spriteCmd.Run()
}

// execOutput runs a command and returns its stdout
func (m *Manager) execOutput(ctx context.Context, cmd string) (string, error) {
	spriteCmd := m.sprite.CommandContext(ctx, "bash", "-c", cmd)

	var stdout, stderr bytes.Buffer
	spriteCmd.Stdout = &stdout
	spriteCmd.Stderr = &stderr

	if err := spriteCmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
