//go:build darwin

package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// ClaudeCredentials represents the Claude Code keychain credentials
type ClaudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	} `json:"claudeAiOauth"`
}

// SpritesConfig represents the sprites.json configuration
type SpritesConfig struct {
	Version          string `json:"version"`
	CurrentSelection struct {
		URL string `json:"url"`
		Org string `json:"org"`
	} `json:"current_selection"`
	Users []struct {
		ID         string `json:"id"`
		Email      string `json:"email"`
		ConfigPath string `json:"config_path"`
		TokenPath  string `json:"token_path"`
	} `json:"users"`
}

// GetTokenFromKeychain retrieves the sprite token from macOS keychain
func GetTokenFromKeychain() (string, error) {
	// First, read the sprites.json to get user ID and org
	configPath := filepath.Join(os.Getenv("HOME"), ".sprites", "sprites.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read sprites config: %w", err)
	}

	var spritesConfig SpritesConfig
	if err := json.Unmarshal(data, &spritesConfig); err != nil {
		return "", fmt.Errorf("failed to parse sprites config: %w", err)
	}

	if len(spritesConfig.Users) == 0 {
		return "", fmt.Errorf("no users found in sprites config")
	}

	// Get the first user's ID
	userID := spritesConfig.Users[0].ID
	org := spritesConfig.CurrentSelection.Org
	url := spritesConfig.CurrentSelection.URL
	if url == "" {
		url = "https://api.sprites.dev"
	}

	// Build the keychain service and account names
	service := fmt.Sprintf("sprites-cli:%s", userID)
	account := fmt.Sprintf("sprites:org:%s:%s", url, org)

	// Try to get token from keychain
	cmd := exec.Command("security", "find-generic-password",
		"-s", service,
		"-a", account,
		"-w")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token from keychain: %w", err)
	}

	tokenStr := strings.TrimSpace(string(output))

	// Handle base64 encoded tokens (go-keyring format)
	if strings.HasPrefix(tokenStr, "go-keyring-base64:") {
		encoded := strings.TrimPrefix(tokenStr, "go-keyring-base64:")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", fmt.Errorf("failed to decode token: %w", err)
		}
		return string(decoded), nil
	}

	return tokenStr, nil
}

// GetTokenFromEnvOrKeychain gets token from env var, config file, or keychain (macOS)
func GetTokenFromEnvOrKeychain() (string, error) {
	// 1. Check environment variable first
	if token := os.Getenv("SPRITE_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Try config file (works on Linux, or when token set via `sprite auth setup --token`)
	config, err := LoadConfig()
	if err == nil {
		if token, err := config.GetToken(); err == nil && token != "" {
			return token, nil
		}
	}

	// 3. Fall back to keychain (macOS only)
	return GetTokenFromKeychain()
}

// GetClaudeTokenFromKeychain retrieves the Claude Code OAuth token from macOS keychain
func GetClaudeTokenFromKeychain() (string, error) {
	// Get current username for the account field
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Claude Code stores credentials with service "Claude Code-credentials" and account as username
	cmd := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials",
		"-a", currentUser.Username,
		"-w")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Claude token from keychain: %w", err)
	}

	// Parse the JSON credentials
	var creds ClaudeCredentials
	if err := json.Unmarshal(output, &creds); err != nil {
		return "", fmt.Errorf("failed to parse Claude credentials: %w", err)
	}

	if creds.ClaudeAiOauth.AccessToken == "" {
		return "", fmt.Errorf("no access token found in Claude credentials")
	}

	return creds.ClaudeAiOauth.AccessToken, nil
}

// GetClaudeToken gets Claude Code OAuth token from env var or keychain
func GetClaudeToken() (string, error) {
	// 1. Check environment variable first (useful for CI/testing)
	if token := os.Getenv("ANTHROPIC_API_KEY"); token != "" {
		return token, nil
	}

	// 2. Fall back to keychain (macOS)
	return GetClaudeTokenFromKeychain()
}
