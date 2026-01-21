package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGitRepo creates a temporary git repository with test files
func setupGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), []byte("content3"), 0644))

	// Add files to git
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

func TestIsGitRepo(t *testing.T) {
	t.Run("valid git repo", func(t *testing.T) {
		tmpDir := setupGitRepo(t)
		assert.True(t, IsGitRepo(tmpDir))
	})

	t.Run("not a git repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.False(t, IsGitRepo(tmpDir))
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		assert.False(t, IsGitRepo("/nonexistent/path"))
	})
}

func TestGetGitTrackedFiles(t *testing.T) {
	tmpDir := setupGitRepo(t)

	files, err := GetGitTrackedFiles(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 3)

	// Check all expected files are present
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	assert.True(t, fileSet["file1.txt"])
	assert.True(t, fileSet["file2.txt"])
	assert.True(t, fileSet["subdir/file3.txt"])
}

func TestGetGitTrackedFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := GetGitTrackedFiles(tmpDir)
	require.Error(t, err)
}

func TestGenerateHandoffID(t *testing.T) {
	tests := []struct {
		name      string
		sourceDir string
		wantDir   string
	}{
		{
			name:      "regular directory",
			sourceDir: "/path/to/my-project",
			wantDir:   "my-project",
		},
		{
			name:      "dot directory",
			sourceDir: ".",
			wantDir:   "handoff",
		},
		{
			name:      "empty string",
			sourceDir: "",
			wantDir:   "handoff",
		},
		{
			name:      "root",
			sourceDir: "/",
			wantDir:   "handoff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateHandoffID(tt.sourceDir)
			// ID should start with the directory name and end with a timestamp
			assert.Contains(t, id, tt.wantDir+"-")
			// Timestamp is 6 digits (HHMMSS)
			assert.Len(t, id, len(tt.wantDir)+1+6)
		})
	}
}

func TestSyncerOptions(t *testing.T) {
	t.Run("WithMaxSize", func(t *testing.T) {
		s := &Syncer{}
		WithMaxSize(1000)(s)
		assert.Equal(t, int64(1000), s.maxSize)
	})

	t.Run("WithVerbose", func(t *testing.T) {
		s := &Syncer{}
		WithVerbose(true)(s)
		assert.True(t, s.verbose)
	})

	t.Run("WithBaseDir", func(t *testing.T) {
		s := &Syncer{}
		WithBaseDir("/custom/path")(s)
		assert.Equal(t, "/custom/path", s.baseDir)
	})

	t.Run("WithHandoffID", func(t *testing.T) {
		s := &Syncer{}
		WithHandoffID("custom-id")(s)
		assert.Equal(t, "custom-id", s.handoffID)
	})

	t.Run("WithProgressCallback", func(t *testing.T) {
		s := &Syncer{}
		called := false
		cb := func(current, total int, path string) {
			called = true
		}
		WithProgressCallback(cb)(s)
		assert.NotNil(t, s.OnProgress)
		s.OnProgress(1, 1, "test")
		assert.True(t, called)
	})
}

func TestSkippedFile(t *testing.T) {
	sf := SkippedFile{
		Path:   "test/file.txt",
		Reason: "file too large",
	}
	assert.Equal(t, "test/file.txt", sf.Path)
	assert.Equal(t, "file too large", sf.Reason)
}

func TestFailedFile(t *testing.T) {
	err := assert.AnError
	ff := FailedFile{
		Path:  "test/file.txt",
		Error: err,
	}
	assert.Equal(t, "test/file.txt", ff.Path)
	assert.Equal(t, err, ff.Error)
}

func TestSyncResult(t *testing.T) {
	result := SyncResult{
		TotalFiles: 10,
		Synced:     7,
		Skipped:    2,
		Failed:     1,
		HandoffID:  "test-123456",
		TargetDir:  "/handoffs/test-123456",
	}

	assert.Equal(t, 10, result.TotalFiles)
	assert.Equal(t, 7, result.Synced)
	assert.Equal(t, 2, result.Skipped)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, "test-123456", result.HandoffID)
	assert.Equal(t, "/handoffs/test-123456", result.TargetDir)
}

func TestMaxFileSize(t *testing.T) {
	// MaxFileSize should be 10MB
	assert.Equal(t, int64(10*1024*1024), int64(MaxFileSize))
}
