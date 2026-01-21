//go:build !darwin

package config

import (
	"fmt"
	"os"
)

// GetTokenFromKeychain is not supported on non-Darwin platforms
func GetTokenFromKeychain() (string, error) {
	return "", ErrTokenNotFound
}

// GetTokenFromEnvOrKeychain gets token from env var or config file (no keychain on non-Darwin)
func GetTokenFromEnvOrKeychain() (string, error) {
	// Check environment variable first
	if token := os.Getenv("SPRITE_TOKEN"); token != "" {
		return token, nil
	}

	// Fall back to config file
	return GetToken()
}

// GetClaudeTokenFromKeychain is not supported on non-Darwin platforms
func GetClaudeTokenFromKeychain() (string, error) {
	return "", fmt.Errorf("keychain not supported on this platform")
}

// GetClaudeToken gets Claude Code OAuth token from env var (keychain not available on non-Darwin)
func GetClaudeToken() (string, error) {
	// Check environment variable
	if token := os.Getenv("ANTHROPIC_API_KEY"); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set (keychain not available on this platform)")
}
