// Package main provides the CLI for sprite-handoff
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/sprites-dev/sprite-handoff/internal/api"
	"github.com/sprites-dev/sprite-handoff/internal/config"
	"github.com/sprites-dev/sprite-handoff/internal/session"
	"github.com/sprites-dev/sprite-handoff/internal/sync"
	"github.com/sprites-dev/sprite-handoff/internal/worktree"
)

var (
	version = "0.1.0"
	verbose bool
)

// shellEscape escapes a string for safe use in shell single quotes.
// It replaces each single quote with '\'' (end quote, escaped quote, start quote).
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "sprite-handoff",
	Short:   "CLI tool for syncing files to Sprites.dev cloud sandboxes",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(claudeTokenCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(writeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
}

// tokenCmd prints the current sprite token
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the current sprite authentication token",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}
		fmt.Println(token)
		return nil
	},
}

// claudeTokenCmd prints the Claude Code OAuth access token
var claudeTokenCmd = &cobra.Command{
	Use:   "claude-token",
	Short: "Print the Claude Code OAuth access token (for passing to remote Claude instances)",
	Long: `Retrieves the Claude Code OAuth access token from the macOS keychain
(or ANTHROPIC_API_KEY environment variable).

This token can be used to authenticate Claude Code running on a remote sprite
by passing it via the ANTHROPIC_API_KEY environment variable.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetClaudeToken()
		if err != nil {
			return err
		}
		fmt.Println(token)
		return nil
	},
}

// syncCmd syncs git-tracked files to a sprite
var syncCmd = &cobra.Command{
	Use:   "sync <sprite-name> [source-dir]",
	Short: "Sync git-tracked files to a sprite",
	Long: `Sync git-tracked files from a local directory to a sprite filesystem.

Files are synced to /home/sprite/handoffs/<dir-name>-HHMMSS/
where <dir-name> is the source directory name.

Examples:
  sprite-handoff sync my-sprite .
  sprite-handoff sync my-sprite ./my-project
  sprite-handoff sync my-sprite . --base-dir /home/sprite/custom`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		sourceDir := "."
		if len(args) > 1 {
			sourceDir = args[1]
		}

		baseDir, _ := cmd.Flags().GetString("base-dir")

		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}

		client := api.NewClient(token)

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		opts := []sync.SyncerOption{
			sync.WithVerbose(verbose),
			sync.WithProgressCallback(func(current, total int, path string) {
				if verbose {
					fmt.Printf("[%d/%d] Syncing: %s\n", current, total, path)
				} else {
					fmt.Printf("\r[%d/%d] Syncing files...", current, total)
				}
			}),
		}

		if baseDir != "" {
			opts = append(opts, sync.WithBaseDir(baseDir))
		}

		syncer := sync.NewSyncer(client, spriteName, sourceDir, opts...)

		result, err := syncer.SyncGitFiles(ctx)
		if err != nil {
			return err
		}

		if !verbose {
			fmt.Println() // New line after progress
		}

		fmt.Printf("\nSync complete:\n")
		fmt.Printf("  Handoff ID: %s\n", result.HandoffID)
		fmt.Printf("  Target:     %s\n", result.TargetDir)
		fmt.Printf("  Total:      %d files\n", result.TotalFiles)
		fmt.Printf("  Synced:     %d files\n", result.Synced)
		fmt.Printf("  Skipped:    %d files\n", result.Skipped)
		fmt.Printf("  Failed:     %d files\n", result.Failed)

		if verbose && len(result.SkippedFiles) > 0 {
			fmt.Println("\nSkipped files:")
			for _, f := range result.SkippedFiles {
				fmt.Printf("  - %s: %s\n", f.Path, f.Reason)
			}
		}

		if len(result.FailedFiles) > 0 {
			fmt.Println("\nFailed files:")
			for _, f := range result.FailedFiles {
				fmt.Printf("  - %s: %v\n", f.Path, f.Error)
			}
			return fmt.Errorf("%d files failed to sync", result.Failed)
		}

		return nil
	},
}

// runCmd syncs files and launches Claude on a sprite
var runCmd = &cobra.Command{
	Use:   "run <sprite-name> <prompt>",
	Short: "Sync files and run Claude Code on a sprite",
	Long: `Sync git-tracked files to a sprite and launch Claude Code with the given prompt.

This command:
1. Creates a git worktree on the sprite for session isolation
2. Syncs all git-tracked files to the worktree
3. Retrieves your Claude Code OAuth token
4. Launches Claude Code on the sprite with the prompt
5. Returns a session ID for resuming later

Examples:
  sprite-handoff run my-sprite "Run the test suite and fix any failures"
  sprite-handoff run my-sprite "Implement the TODO items in main.go" --model opus
  sprite-handoff run my-sprite "Refactor for performance" --max-budget 5

Resume a previous session:
  sprite-handoff run my-sprite "Continue" --resume <session-id>`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		prompt := args[1]

		model, _ := cmd.Flags().GetString("model")
		maxBudget, _ := cmd.Flags().GetFloat64("max-budget")
		sourceDir, _ := cmd.Flags().GetString("source")
		resumeID, _ := cmd.Flags().GetString("resume")

		// Get Claude token for remote authentication
		claudeToken, err := config.GetClaudeToken()
		if err != nil {
			return fmt.Errorf("failed to get Claude token: %w", err)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		// Initialize session store
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		// Get sprite token for API calls
		spriteToken, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return fmt.Errorf("failed to get sprite token: %w", err)
		}

		client := api.NewClient(spriteToken)
		sprite := client.Sprite(spriteName)

		var sess *session.Session
		var targetDir string

		if resumeID != "" {
			// Resume existing session
			fmt.Printf("=== Resuming session %s ===\n", resumeID)

			sess, err = store.Get(resumeID)
			if err != nil {
				return fmt.Errorf("failed to find session: %w", err)
			}

			// Verify worktree still exists
			wtMgr := worktree.NewManager(sprite).WithVerbose(verbose)
			exists, err := wtMgr.WorktreeExists(ctx, sess)
			if err != nil {
				return fmt.Errorf("failed to check worktree: %w", err)
			}
			if !exists {
				return fmt.Errorf("worktree no longer exists: %s", sess.WorktreePath())
			}

			// Update last accessed
			sess.Touch()
			if err := store.Save(sess); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update session: %v\n", err)
			}

			targetDir = sess.WorktreePath()
			fmt.Printf("Using worktree: %s\n", targetDir)
		} else {
			// New session - sync files to worktree
			// Get project name from source directory
			absSourceDir := sourceDir
			if abs, err := os.Getwd(); err == nil && sourceDir == "." {
				absSourceDir = abs
			}
			projectName := getProjectName(absSourceDir)

			sess = session.NewSession(spriteName, projectName, prompt)

			fmt.Printf("=== Creating new session %s ===\n", sess.ID)
			fmt.Printf("Project: %s\n", projectName)

			wtMgr := worktree.NewManager(sprite).WithVerbose(verbose)

			// Initialize bare repo if needed
			fmt.Println("\n=== Initializing repository ===")
			if err := wtMgr.InitRepo(ctx, projectName); err != nil {
				return fmt.Errorf("failed to initialize repo: %w", err)
			}

			// Create worktree
			fmt.Println("Creating worktree...")
			if err := wtMgr.CreateWorktree(ctx, sess); err != nil {
				return fmt.Errorf("failed to create worktree: %w", err)
			}

			// Sync files to worktree
			fmt.Println("\n=== Syncing files ===")
			syncer := sync.NewSyncer(client, spriteName, sourceDir,
				sync.WithVerbose(verbose),
				sync.WithProgressCallback(func(current, total int, path string) {
					if verbose {
						fmt.Printf("[%d/%d] Syncing: %s\n", current, total, path)
					} else {
						fmt.Printf("\r[%d/%d] Syncing files...", current, total)
					}
				}),
			)

			result, err := syncer.SyncToWorktree(ctx, sess)
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			if !verbose {
				fmt.Println() // New line after progress
			}
			fmt.Printf("Synced %d files to %s\n", result.Synced, result.TargetDir)

			// Create initial commit
			fmt.Println("\n=== Creating initial commit ===")
			if err := wtMgr.Commit(ctx, sess, "Initial handoff sync"); err != nil {
				return fmt.Errorf("failed to create initial commit: %w", err)
			}

			// Save session to local store
			if err := store.Save(sess); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}

			// Write session metadata to sprite
			if err := writeRemoteSessionMetadata(ctx, client, spriteName, sess); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write remote session metadata: %v\n", err)
			}

			targetDir = sess.WorktreePath()
		}

		// Launch Claude Code
		fmt.Println("\n=== Launching Claude Code ===")
		fmt.Printf("Session ID: %s\n\n", sess.ID)

		// Build the claude command with flags
		claudeCmd := "claude --print --dangerously-skip-permissions"
		if resumeID != "" {
			claudeCmd += " --resume " + resumeID
		} else {
			claudeCmd += " --session-id " + sess.ID
		}
		claudeCmd += " -p " + shellEscape(prompt)
		if model != "" {
			claudeCmd += fmt.Sprintf(" --model %s", model)
		}
		if maxBudget > 0 {
			claudeCmd += fmt.Sprintf(" --max-budget-usd %.2f", maxBudget)
		}

		// Full command to run on sprite
		remoteCmd := fmt.Sprintf(
			"export CLAUDE_CODE_OAUTH_TOKEN=%q && cd %s && %s",
			claudeToken,
			targetDir,
			claudeCmd,
		)

		if verbose {
			fmt.Printf("Running: sprite exec with TTY\n")
		}

		// Execute via SDK with TTY
		spriteExec := sprite.CommandContext(ctx, "bash", "-c", remoteCmd)
		spriteExec.SetTTY(true)
		spriteExec.Stdout = os.Stdout
		spriteExec.Stderr = os.Stderr
		spriteExec.Stdin = os.Stdin

		err = spriteExec.Run()
		if err != nil {
			return fmt.Errorf("claude execution failed: %w", err)
		}

		fmt.Printf("\n=== Complete ===\n")
		fmt.Printf("Session ID: %s\n", sess.ID)
		fmt.Printf("Worktree:   %s\n", targetDir)
		fmt.Printf("\nTo resume: sprite-handoff run %s \"<prompt>\" --resume %s\n", spriteName, sess.ID)
		fmt.Printf("To pull:   sprite-handoff pull %s\n", sess.ID)
		return nil
	},
}

// getProjectName extracts a project name from a directory path
func getProjectName(dir string) string {
	// Use the base name of the directory
	name := dir
	if abs, err := os.Getwd(); err == nil && (dir == "." || dir == "") {
		name = abs
	}

	base := name
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == '\\' {
			base = name[i+1:]
			break
		}
	}

	if base == "" || base == "." || base == "/" {
		return "project"
	}
	return base
}

// writeRemoteSessionMetadata writes session metadata to the sprite
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

// pullCmd pulls changed files from a handoff session
var pullCmd = &cobra.Command{
	Use:   "pull <session-id>",
	Short: "Pull changed files from a handoff session back to local machine",
	Long: `Pull files that were changed during a handoff session back to the local machine.

By default, creates a local git worktree to isolate the pulled changes, allowing
you to review and merge them. Use --in-place to write directly to the current
directory instead.

This command:
1. Looks up the session by ID
2. Lists files changed in the session's worktree on the sprite
3. Creates a local git worktree (unless --in-place)
4. Downloads each changed file via the filesystem API
5. Commits the changes in the worktree

Examples:
  sprite-handoff pull abc123                    # Pull to local worktree
  sprite-handoff pull abc123 --in-place         # Pull directly to current dir
  sprite-handoff pull abc123 --dry-run          # Preview without writing`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		inPlace, _ := cmd.Flags().GetBool("in-place")

		var outputDir string
		var worktreeBranch string
		var isWorktree bool

		if inPlace {
			// Old behavior: write directly to current directory
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			outputDir = cwd
			isWorktree = false
		} else {
			// New behavior: create a local git worktree
			repoRoot, err := getGitRepoRoot()
			if err != nil {
				return fmt.Errorf("not in a git repository (use --in-place to write directly): %w", err)
			}

			// Create worktree directory
			shortID := sessionID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			worktreeBranch = fmt.Sprintf("sprite-handoff/%s", shortID)
			outputDir = fmt.Sprintf("%s/.sprite-worktrees/%s", repoRoot, shortID)
			isWorktree = true
		}

		// Load session
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		sess, err := store.Get(sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		fmt.Printf("=== Pulling from session %s ===\n", sessionID)
		fmt.Printf("Project:  %s\n", sess.Project)
		fmt.Printf("Sprite:   %s\n", sess.Sprite)
		fmt.Printf("Worktree: %s\n", sess.WorktreePath())
		fmt.Printf("Output:   %s\n\n", outputDir)

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		// Get sprite token
		spriteToken, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return fmt.Errorf("failed to get sprite token: %w", err)
		}

		client := api.NewClient(spriteToken)
		sprite := client.Sprite(sess.Sprite)
		wtMgr := worktree.NewManager(sprite).WithVerbose(verbose)

		// Get list of changed files
		changedFiles, err := wtMgr.ListChangedFiles(ctx, sess)
		if err != nil {
			return fmt.Errorf("failed to list changed files: %w", err)
		}

		if len(changedFiles) == 0 {
			fmt.Println("No changed files to pull.")
			return nil
		}

		fmt.Printf("Found %d changed files:\n", len(changedFiles))
		for _, f := range changedFiles {
			fmt.Printf("  - %s\n", f)
		}

		if dryRun {
			fmt.Println("\n(dry-run mode - no files written)")
			return nil
		}

		// Create local worktree if needed
		if isWorktree {
			repoRoot, _ := getGitRepoRoot() // Already validated above
			fmt.Printf("Creating local worktree at %s...\n", outputDir)
			if err := createLocalWorktree(repoRoot, outputDir, worktreeBranch); err != nil {
				return fmt.Errorf("failed to create local worktree: %w", err)
			}
		}

		fmt.Println()

		// Pull each file
		pulled := 0
		failed := 0
		for i, file := range changedFiles {
			fmt.Printf("[%d/%d] Pulling: %s\n", i+1, len(changedFiles), file)

			// Read file from sprite
			content, err := client.ReadFileWithOptions(ctx, sess.Sprite, file, sess.WorktreePath())
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				failed++
				continue
			}

			// Write to local filesystem
			localPath := outputDir + "/" + file

			// Create parent directory if needed
			dir := getParentDir(localPath)
			if dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "  Error creating directory: %v\n", err)
					failed++
					continue
				}
			}

			if err := os.WriteFile(localPath, content, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "  Error writing file: %v\n", err)
				failed++
				continue
			}

			pulled++
		}

		fmt.Printf("\n=== Pull complete ===\n")
		fmt.Printf("Pulled: %d files\n", pulled)
		if failed > 0 {
			fmt.Printf("Failed: %d files\n", failed)
		}

		// Commit changes and show merge instructions if using worktree
		if isWorktree && pulled > 0 {
			fmt.Println("\nCommitting changes to worktree...")
			commitMsg := fmt.Sprintf("Pull from sprite-handoff session %s", sessionID[:8])
			if err := commitWorktreeChanges(outputDir, commitMsg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to commit changes: %v\n", err)
			}

			fmt.Println("\n=== Next steps ===")
			fmt.Printf("Changes are in worktree: %s\n", outputDir)
			fmt.Printf("Branch: %s\n\n", worktreeBranch)
			fmt.Println("To review changes:")
			fmt.Printf("  cd %s\n", outputDir)
			fmt.Printf("  git diff HEAD~1\n\n")
			fmt.Println("To merge into your current branch:")
			fmt.Printf("  git merge %s\n\n", worktreeBranch)
			fmt.Println("To cherry-pick specific commits:")
			fmt.Printf("  git cherry-pick %s\n\n", worktreeBranch)
			fmt.Println("To clean up the worktree when done:")
			repoRoot, _ := getGitRepoRoot()
			fmt.Printf("  git -C %s worktree remove %s\n", repoRoot, outputDir)
		}

		return nil
	},
}

// getParentDir returns the parent directory of a file path
func getParentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}

// getGitRepoRoot returns the root directory of the current git repository
func getGitRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// createLocalWorktree creates a local git worktree for pulling changes
func createLocalWorktree(repoRoot, worktreePath, branch string) error {
	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, check if it's valid
		checkCmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--git-dir")
		if err := checkCmd.Run(); err == nil {
			// Valid worktree, reuse it
			return nil
		}
		// Invalid, remove and recreate
		os.RemoveAll(worktreePath)
	}

	// Create parent directory
	if err := os.MkdirAll(getParentDir(worktreePath), 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create the worktree with a new branch based on current HEAD
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, worktreePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Branch might already exist, try without -b
		cmd2 := exec.Command("git", "-C", repoRoot, "worktree", "add", worktreePath, branch)
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("failed to create worktree: %s", stderr.String())
		}
	}

	return nil
}

// commitWorktreeChanges commits all changes in a worktree
func commitWorktreeChanges(worktreePath, message string) error {
	// Stage all changes
	addCmd := exec.Command("git", "-C", worktreePath, "add", "-A")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Commit (ignore error if nothing to commit)
	commitCmd := exec.Command("git", "-C", worktreePath, "commit", "-m", message)
	commitCmd.Run() // Ignore error - might be "nothing to commit"

	return nil
}

// sessionsCmd manages handoff sessions
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage handoff sessions",
	Long:  `List, inspect, and clean up handoff sessions.`,
}

// sessionsListCmd lists all sessions
var sessionsListCmd = &cobra.Command{
	Use:   "list [sprite-name]",
	Short: "List handoff sessions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := ""
		if len(args) > 0 {
			spriteName = args[0]
		}

		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		sessions, err := store.List(spriteName)
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		// Print header
		fmt.Printf("%-36s  %-15s  %-20s  %-20s  %s\n", "SESSION", "SPRITE", "PROJECT", "CREATED", "STATUS")
		fmt.Printf("%s  %s  %s  %s  %s\n",
			"------------------------------------",
			"---------------",
			"--------------------",
			"--------------------",
			"----------")

		for _, sess := range sessions {
			created := sess.Created.Format("2006-01-02 15:04")
			fmt.Printf("%-36s  %-15s  %-20s  %-20s  %s\n",
				sess.ID, sess.Sprite, truncate(sess.Project, 20), created, sess.Status)
		}

		return nil
	},
}

// sessionsInfoCmd shows details about a session
var sessionsInfoCmd = &cobra.Command{
	Use:   "info <session-id>",
	Short: "Show details about a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]

		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		sess, err := store.Get(sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		fmt.Printf("Session ID:    %s\n", sess.ID)
		fmt.Printf("Sprite:        %s\n", sess.Sprite)
		fmt.Printf("Project:       %s\n", sess.Project)
		fmt.Printf("Branch:        %s\n", sess.Branch)
		fmt.Printf("Worktree:      %s\n", sess.Worktree)
		fmt.Printf("Status:        %s\n", sess.Status)
		fmt.Printf("Created:       %s\n", sess.Created.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("Last Accessed: %s\n", sess.LastAccessed.Format("2006-01-02 15:04:05 MST"))
		if sess.Prompt != "" {
			fmt.Printf("Prompt:        %s\n", truncate(sess.Prompt, 60))
		}

		return nil
	},
}

// sessionsCleanCmd removes old sessions
var sessionsCleanCmd = &cobra.Command{
	Use:   "clean [sprite-name]",
	Short: "Remove old or abandoned sessions",
	Long: `Remove sessions from the local cache and optionally clean up worktrees on the sprite.

By default, only removes from the local cache. Use --remote to also remove worktrees.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := ""
		if len(args) > 0 {
			spriteName = args[0]
		}
		cleanRemote, _ := cmd.Flags().GetBool("remote")
		sessionID, _ := cmd.Flags().GetString("session")

		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		var sessionsToClean []*session.Session

		if sessionID != "" {
			// Clean specific session
			sess, err := store.Get(sessionID)
			if err != nil {
				return fmt.Errorf("session not found: %w", err)
			}
			sessionsToClean = append(sessionsToClean, sess)
		} else {
			// Get all sessions
			sessions, err := store.List(spriteName)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			sessionsToClean = sessions
		}

		if len(sessionsToClean) == 0 {
			fmt.Println("No sessions to clean.")
			return nil
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		// Get sprite token for API calls if cleaning remote
		var client *api.Client
		if cleanRemote {
			spriteToken, err := config.GetTokenFromEnvOrKeychain()
			if err != nil {
				return fmt.Errorf("failed to get sprite token: %w", err)
			}
			client = api.NewClient(spriteToken)
		}

		cleaned := 0
		for _, sess := range sessionsToClean {
			fmt.Printf("Cleaning session %s...\n", sess.ID)

			// Remove remote worktree if requested
			if cleanRemote && client != nil {
				sprite := client.Sprite(sess.Sprite)
				wtMgr := worktree.NewManager(sprite).WithVerbose(verbose)
				if err := wtMgr.RemoveWorktree(ctx, sess); err != nil {
					fmt.Fprintf(os.Stderr, "  Warning: failed to remove remote worktree: %v\n", err)
				} else {
					fmt.Printf("  Removed worktree: %s\n", sess.WorktreePath())
				}
			}

			// Remove from local store
			if err := store.Delete(sess.ID); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to remove from local store: %v\n", err)
			} else {
				fmt.Printf("  Removed from local cache\n")
				cleaned++
			}
		}

		fmt.Printf("\nCleaned %d sessions.\n", cleaned)
		return nil
	},
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	// Set up sessions subcommands
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsInfoCmd)
	sessionsCmd.AddCommand(sessionsCleanCmd)

	// Add flags
	pullCmd.Flags().Bool("dry-run", false, "show what would be pulled without writing files")
	pullCmd.Flags().Bool("in-place", false, "write files directly to current directory instead of creating a worktree")
	sessionsCleanCmd.Flags().Bool("remote", false, "also remove worktrees from sprites")
	sessionsCleanCmd.Flags().StringP("session", "s", "", "clean a specific session by ID")
}

// readCmd reads a file from a sprite
var readCmd = &cobra.Command{
	Use:   "read <sprite-name> <remote-path>",
	Short: "Read a file from a sprite filesystem",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		remotePath := args[1]

		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}

		client := api.NewClient(token)
		content, err := client.ReadFile(context.Background(), spriteName, remotePath)
		if err != nil {
			return err
		}

		fmt.Print(string(content))
		return nil
	},
}

// writeCmd writes a file to a sprite
var writeCmd = &cobra.Command{
	Use:   "write <sprite-name> <remote-path> <local-path>",
	Short: "Write a local file to a sprite filesystem",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		remotePath := args[1]
		localPath := args[2]

		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}

		client := api.NewClient(token)
		err = client.WriteFileFromPath(context.Background(), spriteName, remotePath, localPath)
		if err != nil {
			return err
		}

		fmt.Printf("Wrote %s to %s:%s\n", localPath, spriteName, remotePath)
		return nil
	},
}

// listCmd lists files in a sprite directory
var listCmd = &cobra.Command{
	Use:   "list <sprite-name> [remote-path]",
	Short: "List files in a sprite directory",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		remotePath := "."
		if len(args) > 1 {
			remotePath = args[1]
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")

		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}

		client := api.NewClient(token)
		entries, err := client.ListDir(context.Background(), spriteName, remotePath)
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		for _, entry := range entries {
			typeChar := "-"
			if entry.IsDir {
				typeChar = "d"
			}
			fmt.Printf("%s %10d %s\n", typeChar, entry.Size, entry.Name)
		}
		return nil
	},
}

// deleteCmd deletes a file from a sprite
var deleteCmd = &cobra.Command{
	Use:   "delete <sprite-name> <remote-path>",
	Short: "Delete a file or directory from a sprite filesystem",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spriteName := args[0]
		remotePath := args[1]
		recursive, _ := cmd.Flags().GetBool("recursive")

		token, err := config.GetTokenFromEnvOrKeychain()
		if err != nil {
			return err
		}

		client := api.NewClient(token)
		err = client.Delete(context.Background(), spriteName, remotePath, api.DeleteOptions{
			Recursive: recursive,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Deleted %s:%s\n", spriteName, remotePath)
		return nil
	},
}

func init() {
	syncCmd.Flags().String("base-dir", "", "base directory for handoffs (default: /handoffs)")
	runCmd.Flags().String("model", "", "Claude model to use (sonnet, opus, haiku)")
	runCmd.Flags().Float64("max-budget", 0, "maximum budget in USD")
	runCmd.Flags().String("source", ".", "source directory to sync")
	runCmd.Flags().StringP("resume", "r", "", "resume a previous session by ID")
	listCmd.Flags().Bool("json", false, "output as JSON")
	deleteCmd.Flags().BoolP("recursive", "r", false, "delete recursively")
}
