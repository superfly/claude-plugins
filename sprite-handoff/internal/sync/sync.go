// Package sync handles syncing local files to sprite filesystems
package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sprites "github.com/superfly/sprites-go"

	"github.com/sprites-dev/sprite-handoff/internal/api"
	"github.com/sprites-dev/sprite-handoff/internal/session"
)

const (
	// MaxFileSize is the maximum file size to sync (10MB)
	MaxFileSize = 10 * 1024 * 1024
)

// SyncResult represents the result of a sync operation
type SyncResult struct {
	TotalFiles   int
	Synced       int
	Skipped      int
	Failed       int
	SkippedFiles []SkippedFile
	FailedFiles  []FailedFile
	HandoffID    string // Unique handoff identifier
	TargetDir    string // Full path to synced files
}

// SkippedFile represents a file that was skipped during sync
type SkippedFile struct {
	Path   string
	Reason string
}

// FailedFile represents a file that failed to sync
type FailedFile struct {
	Path  string
	Error error
}

// Syncer handles file synchronization to sprites
type Syncer struct {
	client     *api.Client
	sprite     *sprites.Sprite // SDK sprite handle
	spriteName string
	sourceDir  string
	baseDir    string // Base directory for handoffs (default: /home/sprite/handoffs)
	handoffID  string // Unique identifier for this handoff
	maxSize    int64
	verbose    bool

	// Callbacks for progress reporting
	OnProgress func(current, total int, path string)
	OnSkip     func(path, reason string)
	OnError    func(path string, err error)
}

// SyncerOption configures the Syncer
type SyncerOption func(*Syncer)

// WithMaxSize sets the maximum file size to sync
func WithMaxSize(size int64) SyncerOption {
	return func(s *Syncer) {
		s.maxSize = size
	}
}

// WithVerbose enables verbose output
func WithVerbose(verbose bool) SyncerOption {
	return func(s *Syncer) {
		s.verbose = verbose
	}
}

// WithProgressCallback sets the progress callback
func WithProgressCallback(fn func(current, total int, path string)) SyncerOption {
	return func(s *Syncer) {
		s.OnProgress = fn
	}
}

// WithBaseDir sets the base directory for handoffs (handoff ID will be appended)
func WithBaseDir(dir string) SyncerOption {
	return func(s *Syncer) {
		s.baseDir = dir
	}
}

// WithHandoffID sets a specific handoff ID (instead of auto-generating)
func WithHandoffID(id string) SyncerOption {
	return func(s *Syncer) {
		s.handoffID = id
	}
}

// WithSprite sets the SDK sprite handle for command execution
func WithSprite(sprite *sprites.Sprite) SyncerOption {
	return func(s *Syncer) {
		s.sprite = sprite
	}
}

// GenerateHandoffID creates a unique handoff identifier
// Format: <dir-name>-HHMMSS (source directory name + timestamp)
func GenerateHandoffID(sourceDir string) string {
	now := time.Now()
	timeSuffix := now.Format("150405")

	// Get the base name of the source directory
	dirName := filepath.Base(sourceDir)
	if dirName == "" || dirName == "." || dirName == "/" {
		// Fallback if we can't get a meaningful name
		dirName = "handoff"
	}

	return fmt.Sprintf("%s-%s", dirName, timeSuffix)
}

// NewSyncer creates a new file syncer
func NewSyncer(client *api.Client, spriteName, sourceDir string, opts ...SyncerOption) *Syncer {
	// Resolve sourceDir to absolute path for meaningful directory name
	absSourceDir := sourceDir
	if abs, err := filepath.Abs(sourceDir); err == nil {
		absSourceDir = abs
	}

	s := &Syncer{
		client:     client,
		spriteName: spriteName,
		sourceDir:  sourceDir,
		baseDir:    api.DefaultHandoffsDir, // /home/sprite/handoffs
		maxSize:    MaxFileSize,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Get sprite handle from client if not provided via option
	if s.sprite == nil {
		s.sprite = client.Sprite(spriteName)
	}

	// Generate handoff ID if not set (uses source directory name + timestamp)
	if s.handoffID == "" {
		s.handoffID = GenerateHandoffID(absSourceDir)
	}

	return s
}

// HandoffID returns the unique identifier for this handoff
func (s *Syncer) HandoffID() string {
	return s.handoffID
}

// TargetDir returns the full target directory (baseDir/handoffID)
func (s *Syncer) TargetDir() string {
	return filepath.Join(s.baseDir, s.handoffID)
}

// GetGitTrackedFiles returns a list of git-tracked files in the directory
func GetGitTrackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// IsGitRepo checks if a directory is a git repository
func IsGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// SyncGitFiles syncs all git-tracked files to the sprite
func (s *Syncer) SyncGitFiles(ctx context.Context) (*SyncResult, error) {
	if !IsGitRepo(s.sourceDir) {
		return nil, fmt.Errorf("not a git repository: %s", s.sourceDir)
	}

	files, err := GetGitTrackedFiles(s.sourceDir)
	if err != nil {
		return nil, err
	}

	return s.SyncFiles(ctx, files)
}

// SyncFiles syncs a list of files to the sprite
func (s *Syncer) SyncFiles(ctx context.Context, files []string) (*SyncResult, error) {
	result := &SyncResult{
		TotalFiles: len(files),
		HandoffID:  s.handoffID,
		TargetDir:  s.TargetDir(),
	}

	// Ensure base directory exists with proper permissions
	// This runs before syncing to create /home/sprite/handoffs with 755 permissions
	if err := s.ensureBaseDirPermissions(ctx); err != nil {
		// Log but don't fail - the sync might still work
		if s.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not set base directory permissions: %v\n", err)
		}
	}

	for i, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if s.OnProgress != nil {
			s.OnProgress(i+1, len(files), file)
		}

		localPath := filepath.Join(s.sourceDir, file)

		// Check if file exists
		info, err := os.Stat(localPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Skipped++
				result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
					Path:   file,
					Reason: "file not found",
				})
				if s.OnSkip != nil {
					s.OnSkip(file, "file not found")
				}
				continue
			}
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, FailedFile{
				Path:  file,
				Error: err,
			})
			if s.OnError != nil {
				s.OnError(file, err)
			}
			continue
		}

		// Skip directories
		if info.IsDir() {
			result.Skipped++
			result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
				Path:   file,
				Reason: "is a directory",
			})
			continue
		}

		// Skip large files
		if info.Size() > s.maxSize {
			result.Skipped++
			result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
				Path:   file,
				Reason: fmt.Sprintf("file too large (%d bytes > %d)", info.Size(), s.maxSize),
			})
			if s.OnSkip != nil {
				s.OnSkip(file, fmt.Sprintf("too large (%d bytes)", info.Size()))
			}
			continue
		}

		// Sync the file to the handoff directory
		err = s.client.WriteFileFromPathWithOptions(ctx, s.spriteName, file, localPath, api.WriteOptions{
			WorkingDir: s.TargetDir(),
		})
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, FailedFile{
				Path:  file,
				Error: err,
			})
			if s.OnError != nil {
				s.OnError(file, err)
			}
			continue
		}

		result.Synced++
	}

	// Fix permissions on all synced files and directories
	if err := s.FixPermissions(ctx); err != nil {
		if s.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not fix permissions: %v\n", err)
		}
	}

	return result, nil
}

// SyncSingleFile syncs a single file to the sprite
func (s *Syncer) SyncSingleFile(ctx context.Context, localPath, remotePath string) error {
	return s.client.WriteFileFromPathWithOptions(ctx, s.spriteName, remotePath, localPath, api.WriteOptions{
		WorkingDir: s.TargetDir(),
	})
}

// ensureBaseDirPermissions creates base directories with proper permissions (755)
// so that all users (including the sprite user) can access them
func (s *Syncer) ensureBaseDirPermissions(ctx context.Context) error {
	// Use SDK to create directories with proper permissions
	// This ensures /home/sprite/handoffs and the target dir are accessible
	cmd := s.sprite.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("mkdir -p %s && chmod 755 %s && mkdir -p %s && chmod 755 %s",
			s.baseDir, s.baseDir, s.TargetDir(), s.TargetDir()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w (output: %s)", err, string(output))
	}
	return nil
}

// FixPermissions ensures all synced files and directories have proper permissions (755/644)
// Call this after syncing if needed
func (s *Syncer) FixPermissions(ctx context.Context) error {
	cmd := s.sprite.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("chmod -R 755 %s", s.TargetDir()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fix permissions: %w (output: %s)", err, string(output))
	}
	return nil
}

// SyncToWorktree syncs git-tracked files to a session's worktree
func (s *Syncer) SyncToWorktree(ctx context.Context, sess *session.Session) (*SyncResult, error) {
	if !IsGitRepo(s.sourceDir) {
		return nil, fmt.Errorf("not a git repository: %s", s.sourceDir)
	}

	files, err := GetGitTrackedFiles(s.sourceDir)
	if err != nil {
		return nil, err
	}

	return s.SyncFilesToWorktree(ctx, sess, files)
}

// SyncFilesToWorktree syncs a list of files to a session's worktree
func (s *Syncer) SyncFilesToWorktree(ctx context.Context, sess *session.Session, files []string) (*SyncResult, error) {
	result := &SyncResult{
		TotalFiles: len(files),
		HandoffID:  sess.ID,
		TargetDir:  sess.WorktreePath(),
	}

	// Ensure worktree directory has proper permissions
	if err := s.ensureWorktreePermissions(ctx, sess); err != nil {
		if s.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not set worktree permissions: %v\n", err)
		}
	}

	for i, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if s.OnProgress != nil {
			s.OnProgress(i+1, len(files), file)
		}

		localPath := filepath.Join(s.sourceDir, file)

		// Check if file exists
		info, err := os.Stat(localPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Skipped++
				result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
					Path:   file,
					Reason: "file not found",
				})
				if s.OnSkip != nil {
					s.OnSkip(file, "file not found")
				}
				continue
			}
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, FailedFile{
				Path:  file,
				Error: err,
			})
			if s.OnError != nil {
				s.OnError(file, err)
			}
			continue
		}

		// Skip directories
		if info.IsDir() {
			result.Skipped++
			result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
				Path:   file,
				Reason: "is a directory",
			})
			continue
		}

		// Skip large files
		if info.Size() > s.maxSize {
			result.Skipped++
			result.SkippedFiles = append(result.SkippedFiles, SkippedFile{
				Path:   file,
				Reason: fmt.Sprintf("file too large (%d bytes > %d)", info.Size(), s.maxSize),
			})
			if s.OnSkip != nil {
				s.OnSkip(file, fmt.Sprintf("too large (%d bytes)", info.Size()))
			}
			continue
		}

		// Sync the file to the worktree directory
		err = s.client.WriteFileFromPathWithOptions(ctx, s.spriteName, file, localPath, api.WriteOptions{
			WorkingDir: sess.WorktreePath(),
		})
		if err != nil {
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, FailedFile{
				Path:  file,
				Error: err,
			})
			if s.OnError != nil {
				s.OnError(file, err)
			}
			continue
		}

		result.Synced++
	}

	// Fix permissions on all synced files
	if err := s.fixWorktreePermissions(ctx, sess); err != nil {
		if s.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not fix worktree permissions: %v\n", err)
		}
	}

	return result, nil
}

// ensureWorktreePermissions creates worktree directory with proper permissions
func (s *Syncer) ensureWorktreePermissions(ctx context.Context, sess *session.Session) error {
	cmd := s.sprite.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("chmod 755 %s 2>/dev/null || true", sess.WorktreePath()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w (output: %s)", err, string(output))
	}
	return nil
}

// fixWorktreePermissions ensures all files in worktree have proper permissions
func (s *Syncer) fixWorktreePermissions(ctx context.Context, sess *session.Session) error {
	cmd := s.sprite.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("chmod -R 755 %s", sess.WorktreePath()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fix permissions: %w (output: %s)", err, string(output))
	}
	return nil
}
