// Package api provides a client for the Sprites.dev Filesystem API
package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	sprites "github.com/superfly/sprites-go"
)

const (
	DefaultHandoffsDir = "/handoffs"
)

// Client is a Sprites.dev API client wrapping the sprites-go SDK
type Client struct {
	sdk *sprites.Client
}

// ClientOption configures the client (kept for backward compatibility)
type ClientOption func(*Client)

// NewClient creates a new Sprites API client using the SDK
func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		sdk: sprites.New(token),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Sprite returns a sprite handle from the SDK
func (c *Client) Sprite(name string) *sprites.Sprite {
	return c.sdk.Sprite(name)
}

// FileInfo represents information about a file or directory
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
}

// WriteOptions configures file write behavior
type WriteOptions struct {
	WorkingDir string
	Mode       string
	MkdirAll   bool
}

// WriteFile writes content to a file on the sprite filesystem
func (c *Client) WriteFile(ctx context.Context, spriteName, remotePath string, content []byte) error {
	return c.WriteFileWithOptions(ctx, spriteName, remotePath, content, WriteOptions{})
}

// WriteFileWithOptions writes content with additional options
func (c *Client) WriteFileWithOptions(ctx context.Context, spriteName, remotePath string, content []byte, opts WriteOptions) error {
	workingDir := opts.WorkingDir
	if workingDir == "" {
		workingDir = DefaultHandoffsDir
	}

	sprite := c.sdk.Sprite(spriteName)
	sfs := sprite.FilesystemAt(workingDir)

	// Parse mode if provided, default to 0644
	var mode fs.FileMode = 0644
	if opts.Mode != "" {
		var parsed uint32
		// Try parsing as octal
		_, err := parseOctalMode(opts.Mode)
		if err == nil {
			mode = fs.FileMode(parsed)
		}
	}

	// Ensure parent directory exists if MkdirAll is set
	if opts.MkdirAll {
		dir := filepath.Dir(remotePath)
		if dir != "" && dir != "." {
			if err := sfs.MkdirAll(dir, 0755); err != nil {
				// Ignore errors - directory might already exist
			}
		}
	}

	return sfs.WriteFile(remotePath, content, mode)
}

// parseOctalMode parses a mode string like "0755" into fs.FileMode
func parseOctalMode(s string) (fs.FileMode, error) {
	var mode uint32
	_, err := parseOctal(s, &mode)
	return fs.FileMode(mode), err
}

func parseOctal(s string, v *uint32) (int, error) {
	var result uint32
	for i, c := range s {
		if c < '0' || c > '7' {
			return i, os.ErrInvalid
		}
		result = result*8 + uint32(c-'0')
	}
	*v = result
	return len(s), nil
}

// WriteFileFromPath writes a local file to the sprite filesystem
func (c *Client) WriteFileFromPath(ctx context.Context, spriteName, remotePath, localPath string) error {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	return c.WriteFileWithOptions(ctx, spriteName, remotePath, content, WriteOptions{
		MkdirAll: true,
	})
}

// WriteFileFromPathWithOptions writes a local file with custom options
func (c *Client) WriteFileFromPathWithOptions(ctx context.Context, spriteName, remotePath, localPath string, opts WriteOptions) error {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	opts.MkdirAll = true
	return c.WriteFileWithOptions(ctx, spriteName, remotePath, content, opts)
}

// ReadFile reads a file from the sprite filesystem
func (c *Client) ReadFile(ctx context.Context, spriteName, remotePath string) ([]byte, error) {
	return c.ReadFileWithOptions(ctx, spriteName, remotePath, DefaultHandoffsDir)
}

// ReadFileWithOptions reads a file with custom working directory
func (c *Client) ReadFileWithOptions(ctx context.Context, spriteName, remotePath, workingDir string) ([]byte, error) {
	sprite := c.sdk.Sprite(spriteName)
	sfs := sprite.FilesystemAt(workingDir)
	return sfs.ReadFile(remotePath)
}

// ListDir lists the contents of a directory
func (c *Client) ListDir(ctx context.Context, spriteName, remotePath string) ([]FileInfo, error) {
	return c.ListDirWithOptions(ctx, spriteName, remotePath, DefaultHandoffsDir)
}

// ListDirWithOptions lists directory contents with custom working directory
func (c *Client) ListDirWithOptions(ctx context.Context, spriteName, remotePath, workingDir string) ([]FileInfo, error) {
	sprite := c.sdk.Sprite(spriteName)
	sfs := sprite.FilesystemAt(workingDir)

	entries, err := sfs.ReadDir(remotePath)
	if err != nil {
		return nil, err
	}

	var result []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(remotePath, entry.Name()),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			IsDir:   entry.IsDir(),
		})
	}

	return result, nil
}

// DeleteOptions configures delete behavior
type DeleteOptions struct {
	WorkingDir string
	Recursive  bool
	AsRoot     bool
}

// Delete removes a file or directory
func (c *Client) Delete(ctx context.Context, spriteName, remotePath string, opts DeleteOptions) error {
	workingDir := opts.WorkingDir
	if workingDir == "" {
		workingDir = DefaultHandoffsDir
	}

	sprite := c.sdk.Sprite(spriteName)
	sfs := sprite.FilesystemAt(workingDir)

	if opts.Recursive {
		return sfs.RemoveAll(remotePath)
	}
	return sfs.Remove(remotePath)
}

// Exists checks if a file or directory exists
func (c *Client) Exists(ctx context.Context, spriteName, remotePath string) (bool, error) {
	_, err := c.ReadFile(ctx, spriteName, remotePath)
	if err != nil {
		// Check if it's a directory by trying to list it
		_, listErr := c.ListDir(ctx, spriteName, remotePath)
		if listErr != nil {
			return false, nil // Neither file nor directory exists
		}
		return true, nil // It's a directory
	}
	return true, nil // File exists
}
