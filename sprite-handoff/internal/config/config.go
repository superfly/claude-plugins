// Package config handles sprite CLI configuration and authentication
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrConfigNotFound = errors.New("sprite config not found - run 'sprite org auth' first")
	ErrTokenNotFound  = errors.New("no token found in sprite config")
)

// Config represents the sprite CLI configuration
type Config struct {
	Token string `json:"token"`
	Auth  *Auth  `json:"auth,omitempty"`
}

// Auth represents alternative auth structure
type Auth struct {
	Token string `json:"token"`
}

// DefaultConfigPath returns the default sprite config file path
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".sprites", "sprites.json")
}

// LoadConfig loads the sprite configuration from the default path
func LoadConfig() (*Config, error) {
	return LoadConfigFrom(DefaultConfigPath())
}

// LoadConfigFrom loads configuration from a specific path
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// GetToken retrieves the authentication token from config
func (c *Config) GetToken() (string, error) {
	// Try direct token field first
	if c.Token != "" {
		return c.Token, nil
	}

	// Try nested auth.token
	if c.Auth != nil && c.Auth.Token != "" {
		return c.Auth.Token, nil
	}

	return "", ErrTokenNotFound
}

// GetToken is a convenience function to get the token from default config
func GetToken() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return config.GetToken()
}

// GetTokenFromEnvOrConfig gets token from SPRITE_TOKEN env var or config file
func GetTokenFromEnvOrConfig() (string, error) {
	// Check environment variable first
	if token := os.Getenv("SPRITE_TOKEN"); token != "" {
		return token, nil
	}

	// Fall back to config file
	return GetToken()
}
