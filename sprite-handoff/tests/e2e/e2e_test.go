// Package e2e contains end-to-end tests that test the full sprite-handoff workflow
// These tests require a real sprite and will create/modify/delete files
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sprites-dev/sprite-handoff/internal/api"
	"github.com/sprites-dev/sprite-handoff/internal/config"
	"github.com/sprites-dev/sprite-handoff/internal/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_FullSyncWorkflow tests the complete workflow:
// 1. Create a local git repo with files
// 2. Sync all files to a sprite
// 3. Verify files exist on sprite
// 4. Clean up
func TestE2E_FullSyncWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	token, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		t.Skipf("Skipping E2E test: %v", err)
	}

	spriteName := os.Getenv("SPRITE_NAME")
	if spriteName == "" {
		t.Skip("Skipping E2E test: SPRITE_NAME not set")
	}

	ctx := context.Background()
	client := api.NewClient(token)

	// Create unique test directory on sprite
	testPrefix := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())

	// Setup: Create local git repo with test files
	tmpDir := setupTestGitRepo(t)

	// Track synced files for verification
	var syncedFiles []string

	t.Run("SyncFiles", func(t *testing.T) {
		// Get git-tracked files and sync with test prefix
		files, err := sync.GetGitTrackedFiles(tmpDir)
		require.NoError(t, err)

		// Sync files with test prefix
		for _, file := range files {
			localPath := filepath.Join(tmpDir, file)
			remotePath := testPrefix + "/" + file

			content, err := os.ReadFile(localPath)
			require.NoError(t, err)

			err = client.WriteFileWithOptions(ctx, spriteName, remotePath, content, api.WriteOptions{
				MkdirAll: true,
			})
			require.NoError(t, err)

			syncedFiles = append(syncedFiles, remotePath)
		}

		t.Logf("Synced %d files to sprite", len(syncedFiles))
	})

	t.Run("VerifyFilesExist", func(t *testing.T) {
		for _, remotePath := range syncedFiles {
			exists, err := client.Exists(ctx, spriteName, remotePath)
			require.NoError(t, err)
			assert.True(t, exists, "file should exist on sprite: %s", remotePath)
		}
	})

	t.Run("VerifyFileContents", func(t *testing.T) {
		// Check specific file content
		content, err := client.ReadFile(ctx, spriteName, testPrefix+"/README.md")
		require.NoError(t, err)
		assert.Contains(t, string(content), "Test Project")
	})

	t.Run("ListSyncedDirectory", func(t *testing.T) {
		entries, err := client.ListDir(ctx, spriteName, testPrefix)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)

		t.Logf("Directory listing for %s:", testPrefix)
		for _, entry := range entries {
			typeChar := "f"
			if entry.IsDir {
				typeChar = "d"
			}
			t.Logf("  [%s] %s (%d bytes)", typeChar, entry.Name, entry.Size)
		}
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		err := client.Delete(ctx, spriteName, testPrefix, api.DeleteOptions{
			Recursive: true,
		})
		require.NoError(t, err)
		t.Logf("Cleaned up test directory: %s", testPrefix)
	})
}

// TestE2E_CLICommands tests the CLI commands directly
func TestE2E_CLICommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Check if binary exists
	binaryPath := findBinary(t)
	if binaryPath == "" {
		t.Skip("sprite-handoff binary not found, run 'go build' first")
	}

	spriteName := os.Getenv("SPRITE_NAME")
	if spriteName == "" {
		t.Skip("Skipping E2E test: SPRITE_NAME not set")
	}

	testFile := fmt.Sprintf("cli-test-%d.txt", time.Now().UnixNano())
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "local.txt")
	require.NoError(t, os.WriteFile(localFile, []byte("CLI test content"), 0644))

	t.Run("token", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "token")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "token command failed: %s", output)
		assert.NotEmpty(t, output)
		t.Logf("Token retrieved (length: %d)", len(output))
	})

	t.Run("write", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "write", spriteName, testFile, localFile)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "write command failed: %s", output)
		assert.Contains(t, string(output), "Wrote")
	})

	t.Run("read", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "read", spriteName, testFile)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "read command failed: %s", output)
		assert.Equal(t, "CLI test content", string(output))
	})

	t.Run("list", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "list", spriteName, ".")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "list command failed: %s", output)
		assert.Contains(t, string(output), testFile)
	})

	t.Run("list-json", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "list", "--json", spriteName, ".")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "list --json command failed: %s", output)
		assert.Contains(t, string(output), `"name"`)
	})

	t.Run("delete", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "delete", spriteName, testFile)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "delete command failed: %s", output)
		assert.Contains(t, string(output), "Deleted")
	})
}

// TestE2E_SyncCommand tests the sync command with a real git repo
func TestE2E_SyncCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	binaryPath := findBinary(t)
	if binaryPath == "" {
		t.Skip("sprite-handoff binary not found, run 'go build' first")
	}

	spriteName := os.Getenv("SPRITE_NAME")
	if spriteName == "" {
		t.Skip("Skipping E2E test: SPRITE_NAME not set")
	}

	tmpDir := setupTestGitRepo(t)

	t.Run("sync", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "sync", "-v", spriteName, tmpDir)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "sync command failed: %s", output)

		outputStr := string(output)
		assert.Contains(t, outputStr, "Sync complete")
		assert.Contains(t, outputStr, "Synced:")
		t.Log(outputStr)
	})
}

// Helper functions

func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create test files
	files := map[string]string{
		"README.md":           "# Test Project\n\nThis is a test project for E2E testing.\n",
		"src/main.go":         "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}\n",
		"src/utils/helper.go": "package utils\n\nfunc Helper() string {\n\treturn \"helper\"\n}\n",
		"config.json":         `{"name": "test", "version": "1.0.0"}`,
		".gitignore":          "*.log\n*.tmp\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Add and commit
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	return tmpDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, output)
}

func findBinary(t *testing.T) string {
	t.Helper()

	// Try common locations
	locations := []string{
		"../../sprite-handoff",
		"../../cmd/sprite-handoff/sprite-handoff",
		"sprite-handoff",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			absPath, _ := filepath.Abs(loc)
			return absPath
		}
	}

	// Try to find via go build output
	cmd := exec.Command("go", "build", "-o", "/dev/null", "../../cmd/sprite-handoff")
	if err := cmd.Run(); err == nil {
		// Binary would be built, but we sent to /dev/null
		// Try building to temp location
		tmpBinary := filepath.Join(t.TempDir(), "sprite-handoff")
		cmd = exec.Command("go", "build", "-o", tmpBinary, "../../cmd/sprite-handoff")
		if err := cmd.Run(); err == nil {
			return tmpBinary
		}
	}

	return ""
}
