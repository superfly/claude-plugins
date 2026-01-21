package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFrom(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "direct token field",
			content:   `{"token": "org/id/token-id/secret-value"}`,
			wantToken: "org/id/token-id/secret-value",
		},
		{
			name:      "nested auth.token field",
			content:   `{"auth": {"token": "nested-token-value"}}`,
			wantToken: "nested-token-value",
		},
		{
			name:      "both fields - direct takes precedence",
			content:   `{"token": "direct", "auth": {"token": "nested"}}`,
			wantToken: "direct",
		},
		{
			name:        "empty config - no token",
			content:     `{}`,
			wantErr:     true,
			errContains: "no token found",
		},
		{
			name:        "invalid JSON",
			content:     `{invalid`,
			wantErr:     true,
			errContains: "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with config content
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "sprites.json")
			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			config, err := LoadConfigFrom(configPath)

			if tt.wantErr {
				if err == nil {
					// Config loaded, but GetToken should fail
					_, tokenErr := config.GetToken()
					require.Error(t, tokenErr)
					assert.Contains(t, tokenErr.Error(), tt.errContains)
				} else {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			token, err := config.GetToken()
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestLoadConfigFrom_FileNotFound(t *testing.T) {
	_, err := LoadConfigFrom("/nonexistent/path/sprites.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestGetTokenFromEnvOrConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".sprites", "sprites.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"token": "config-token"}`), 0644))

	// Test with env var set
	t.Run("env var takes precedence", func(t *testing.T) {
		os.Setenv("SPRITE_TOKEN", "env-token")
		defer os.Unsetenv("SPRITE_TOKEN")

		token, err := GetTokenFromEnvOrConfig()
		require.NoError(t, err)
		assert.Equal(t, "env-token", token)
	})

	// Test without env var - falls back to config
	// Note: This test depends on the actual home directory config
	// In real tests, you'd mock DefaultConfigPath
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	assert.Contains(t, path, ".sprites")
	assert.Contains(t, path, "sprites.json")
}

func TestConfig_GetToken_EmptyConfig(t *testing.T) {
	config := &Config{}
	_, err := config.GetToken()
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

func TestConfig_GetToken_EmptyAuth(t *testing.T) {
	config := &Config{
		Auth: &Auth{},
	}
	_, err := config.GetToken()
	assert.ErrorIs(t, err, ErrTokenNotFound)
}
