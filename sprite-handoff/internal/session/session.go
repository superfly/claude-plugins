// Package session manages handoff session metadata
package session

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"time"
)

const (
	// StatusActive indicates the session is currently in use
	StatusActive = "active"
	// StatusCompleted indicates the session finished successfully
	StatusCompleted = "completed"
	// StatusAbandoned indicates the session was abandoned
	StatusAbandoned = "abandoned"

	// HandoffsDir is the base directory for all handoffs on sprites
	HandoffsDir = "/handoffs"
	// SessionsMetadataDir is where session metadata files are stored
	SessionsMetadataDir = "/handoffs/.sessions"
)

// Session represents a handoff session
type Session struct {
	ID           string    `json:"id"`
	Sprite       string    `json:"sprite"`
	Project      string    `json:"project"`
	Worktree     string    `json:"worktree"`
	Branch       string    `json:"branch"`
	Created      time.Time `json:"created"`
	LastAccessed time.Time `json:"lastAccessed"`
	Prompt       string    `json:"prompt"`
	Status       string    `json:"status"`
}

// NewSession creates a new session with a generated UUID
func NewSession(sprite, project, prompt string) *Session {
	id := generateUUID()
	now := time.Now().UTC()

	return &Session{
		ID:           id,
		Sprite:       sprite,
		Project:      project,
		Worktree:     worktreePath(project, id),
		Branch:       fmt.Sprintf("handoff/%s", id),
		Created:      now,
		LastAccessed: now,
		Prompt:       prompt,
		Status:       StatusActive,
	}
}

// WorktreePath returns the full path to the worktree directory
func (s *Session) WorktreePath() string {
	return s.Worktree
}

// RepoPath returns the path to the bare git repo for this project
func (s *Session) RepoPath() string {
	return filepath.Join(HandoffsDir, s.Project)
}

// WorktreesDir returns the directory containing all worktrees for this project
func (s *Session) WorktreesDir() string {
	return filepath.Join(HandoffsDir, s.Project+".worktrees")
}

// MetadataPath returns the path to the session metadata file on the sprite
func (s *Session) MetadataPath() string {
	return filepath.Join(SessionsMetadataDir, s.ID+".json")
}

// Touch updates the LastAccessed timestamp
func (s *Session) Touch() {
	s.LastAccessed = time.Now().UTC()
}

// worktreePath generates the worktree path for a project and session ID
func worktreePath(project, sessionID string) string {
	return filepath.Join(HandoffsDir, project+".worktrees", sessionID)
}

// generateUUID creates a random UUID v4
func generateUUID() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
