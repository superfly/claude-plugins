package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	assert.NotNil(t, client)
	assert.NotNil(t, client.sdk)
}

func TestClient_Sprite(t *testing.T) {
	client := NewClient("test-token")
	sprite := client.Sprite("my-sprite")
	assert.NotNil(t, sprite)
}

func TestWriteOptions_Defaults(t *testing.T) {
	opts := WriteOptions{}
	assert.Empty(t, opts.WorkingDir)
	assert.Empty(t, opts.Mode)
	assert.False(t, opts.MkdirAll)
}

func TestDeleteOptions_Defaults(t *testing.T) {
	opts := DeleteOptions{}
	assert.Empty(t, opts.WorkingDir)
	assert.False(t, opts.Recursive)
	assert.False(t, opts.AsRoot)
}

func TestFileInfo_Fields(t *testing.T) {
	info := FileInfo{
		Name:    "test.txt",
		Path:    "/path/to/test.txt",
		Size:    100,
		Mode:    "-rw-r--r--",
		ModTime: "2024-01-01T00:00:00Z",
		IsDir:   false,
	}

	assert.Equal(t, "test.txt", info.Name)
	assert.Equal(t, "/path/to/test.txt", info.Path)
	assert.Equal(t, int64(100), info.Size)
	assert.Equal(t, "-rw-r--r--", info.Mode)
	assert.Equal(t, "2024-01-01T00:00:00Z", info.ModTime)
	assert.False(t, info.IsDir)
}

func TestParseOctalMode(t *testing.T) {
	tests := []struct {
		input    string
		expected uint32
		hasError bool
	}{
		{"755", 0755, false},
		{"644", 0644, false},
		{"777", 0777, false},
		{"0755", 0755, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := parseOctalMode(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, uint32(mode))
			}
		})
	}
}
