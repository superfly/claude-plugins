// Package integration contains integration tests that run against the real Sprites.dev API
// These tests require SPRITE_TOKEN and SPRITE_NAME environment variables
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sprites-dev/sprite-handoff/internal/api"
	"github.com/sprites-dev/sprite-handoff/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestClient(t *testing.T) (*api.Client, string) {
	t.Helper()

	token, err := config.GetTokenFromEnvOrKeychain()
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}

	spriteName := os.Getenv("SPRITE_NAME")
	if spriteName == "" {
		t.Skip("Skipping integration test: SPRITE_NAME not set")
	}

	return api.NewClient(token), spriteName
}

func TestIntegration_WriteReadDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, spriteName := getTestClient(t)
	ctx := context.Background()

	testFile := fmt.Sprintf("test-file-%d.txt", time.Now().UnixNano())
	testContent := []byte("Hello from integration test!")

	// Write file
	t.Run("WriteFile", func(t *testing.T) {
		err := client.WriteFile(ctx, spriteName, testFile, testContent)
		require.NoError(t, err)
	})

	// Read file back
	t.Run("ReadFile", func(t *testing.T) {
		content, err := client.ReadFile(ctx, spriteName, testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content)
	})

	// List directory should contain the file
	t.Run("ListDir", func(t *testing.T) {
		entries, err := client.ListDir(ctx, spriteName, ".")
		require.NoError(t, err)

		found := false
		for _, entry := range entries {
			if entry.Name == testFile {
				found = true
				assert.False(t, entry.IsDir)
				assert.Equal(t, int64(len(testContent)), entry.Size)
				break
			}
		}
		assert.True(t, found, "test file should be in directory listing")
	})

	// Clean up - delete file
	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(ctx, spriteName, testFile, api.DeleteOptions{})
		require.NoError(t, err)
	})

	// Verify deletion
	t.Run("VerifyDeleted", func(t *testing.T) {
		_, err := client.ReadFile(ctx, spriteName, testFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestIntegration_WriteWithMkdir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, spriteName := getTestClient(t)
	ctx := context.Background()

	testDir := fmt.Sprintf("test-dir-%d", time.Now().UnixNano())
	testFile := testDir + "/subdir/nested/file.txt"
	testContent := []byte("Nested file content")

	// Write file with mkdir=true
	t.Run("WriteNestedFile", func(t *testing.T) {
		err := client.WriteFileWithOptions(ctx, spriteName, testFile, testContent, api.WriteOptions{
			MkdirAll: true,
		})
		require.NoError(t, err)
	})

	// Read it back
	t.Run("ReadNestedFile", func(t *testing.T) {
		content, err := client.ReadFile(ctx, spriteName, testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content)
	})

	// Clean up
	t.Run("Cleanup", func(t *testing.T) {
		err := client.Delete(ctx, spriteName, testDir, api.DeleteOptions{
			Recursive: true,
		})
		require.NoError(t, err)
	})
}

func TestIntegration_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, spriteName := getTestClient(t)
	ctx := context.Background()

	testFile := fmt.Sprintf("large-file-%d.bin", time.Now().UnixNano())

	// Create 1MB of data
	testContent := make([]byte, 1024*1024)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}

	// Write large file
	t.Run("WriteLargeFile", func(t *testing.T) {
		err := client.WriteFile(ctx, spriteName, testFile, testContent)
		require.NoError(t, err)
	})

	// Read it back and verify
	t.Run("ReadLargeFile", func(t *testing.T) {
		content, err := client.ReadFile(ctx, spriteName, testFile)
		require.NoError(t, err)
		assert.Equal(t, len(testContent), len(content))
		assert.Equal(t, testContent, content)
	})

	// Clean up
	t.Run("Cleanup", func(t *testing.T) {
		err := client.Delete(ctx, spriteName, testFile, api.DeleteOptions{})
		require.NoError(t, err)
	})
}

func TestIntegration_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, spriteName := getTestClient(t)

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// This should fail due to timeout
	err := client.WriteFile(ctx, spriteName, "timeout-test.txt", []byte("content"))
	require.Error(t, err)
}
