package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store manages local session cache
type Store struct {
	path string
	mu   sync.RWMutex
}

// StoreData is the JSON structure for the local cache file
type StoreData struct {
	Sessions map[string]*Session `json:"sessions"`
}

// NewStore creates a new session store
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storePath := filepath.Join(homeDir, ".sprite-handoff", "sessions.json")

	// Ensure directory exists
	dir := filepath.Dir(storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	return &Store{path: storePath}, nil
}

// NewStoreWithPath creates a store with a custom path (for testing)
func NewStoreWithPath(path string) *Store {
	return &Store{path: path}
}

// Save saves a session to the local cache
func (s *Store) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		data = &StoreData{Sessions: make(map[string]*Session)}
	}

	data.Sessions[session.ID] = session

	return s.save(data)
}

// Get retrieves a session by ID
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.load()
	if err != nil {
		return nil, err
	}

	session, ok := data.Sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// List returns all sessions, optionally filtered by sprite name
func (s *Store) List(spriteName string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.load()
	if err != nil {
		// Return empty list if no cache exists yet
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	var sessions []*Session
	for _, session := range data.Sessions {
		if spriteName == "" || session.Sprite == spriteName {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// Delete removes a session from the local cache
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return err
	}

	delete(data.Sessions, id)

	return s.save(data)
}

// UpdateStatus updates the status of a session
func (s *Store) UpdateStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return err
	}

	session, ok := data.Sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.Status = status
	session.Touch()

	return s.save(data)
}

// load reads the store data from disk
func (s *Store) load() (*StoreData, error) {
	content, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var data StoreData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse session store: %w", err)
	}

	if data.Sessions == nil {
		data.Sessions = make(map[string]*Session)
	}

	return &data, nil
}

// save writes the store data to disk
func (s *Store) save(data *StoreData) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session store: %w", err)
	}

	// Write atomically using temp file
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write session store: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save session store: %w", err)
	}

	return nil
}
