// Package mcpserver provides an MCP server for sprite-handoff operations.
package mcpserver

// SpriteRunInput contains parameters for running Claude on a sprite.
type SpriteRunInput struct {
	SpriteName string  `json:"sprite_name" jsonschema:"The name of the sprite to run on"`
	Prompt     string  `json:"prompt" jsonschema:"The prompt to send to Claude"`
	SourceDir  string  `json:"source_dir,omitempty" jsonschema:"Source directory to sync (default: current directory)"`
	Model      string  `json:"model,omitempty" jsonschema:"Claude model to use (sonnet, opus, haiku)"`
	ResumeID   string  `json:"resume_id,omitempty" jsonschema:"Session ID to resume instead of creating new"`
	MaxBudget  float64 `json:"max_budget,omitempty" jsonschema:"Maximum budget in USD"`
}

// SpriteRunOutput contains the result of a sprite run operation.
type SpriteRunOutput struct {
	SessionID    string `json:"session_id"`
	WorktreePath string `json:"worktree_path"`
	FilesSynced  int    `json:"files_synced"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

// SpritePullInput contains parameters for pulling files from a session.
type SpritePullInput struct {
	SessionID string `json:"session_id" jsonschema:"The session ID to pull from"`
	DryRun    bool   `json:"dry_run,omitempty" jsonschema:"Preview without writing files"`
	InPlace   bool   `json:"in_place,omitempty" jsonschema:"Write directly to current directory instead of worktree"`
}

// SpritePullOutput contains the result of a pull operation.
type SpritePullOutput struct {
	FilesPulled   int      `json:"files_pulled"`
	FilesFailed   int      `json:"files_failed"`
	ChangedFiles  []string `json:"changed_files"`
	OutputDir     string   `json:"output_dir"`
	WorktreePath  string   `json:"worktree_path,omitempty"`
	Branch        string   `json:"branch,omitempty"`
}

// SessionsListInput contains parameters for listing sessions.
type SessionsListInput struct {
	SpriteName string `json:"sprite_name,omitempty" jsonschema:"Filter sessions by sprite name"`
}

// SessionInfo represents a single session in list output.
type SessionInfo struct {
	ID           string `json:"id"`
	Sprite       string `json:"sprite"`
	Project      string `json:"project"`
	Status       string `json:"status"`
	Created      string `json:"created"`
	LastAccessed string `json:"last_accessed"`
	Prompt       string `json:"prompt,omitempty"`
}

// SessionsListOutput contains the list of sessions.
type SessionsListOutput struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// SessionsInfoInput contains parameters for getting session details.
type SessionsInfoInput struct {
	SessionID string `json:"session_id" jsonschema:"The session ID to get info for"`
}

// SessionsInfoOutput contains detailed session information.
type SessionsInfoOutput struct {
	ID           string `json:"id"`
	Sprite       string `json:"sprite"`
	Project      string `json:"project"`
	Branch       string `json:"branch"`
	Worktree     string `json:"worktree"`
	Status       string `json:"status"`
	Created      string `json:"created"`
	LastAccessed string `json:"last_accessed"`
	Prompt       string `json:"prompt"`
}

// SessionsCleanInput contains parameters for cleaning sessions.
type SessionsCleanInput struct {
	SpriteName string `json:"sprite_name,omitempty" jsonschema:"Filter sessions by sprite name"`
	SessionID  string `json:"session_id,omitempty" jsonschema:"Clean a specific session by ID"`
	Remote     bool   `json:"remote,omitempty" jsonschema:"Also remove worktrees from sprites"`
}

// SessionsCleanOutput contains the result of cleaning sessions.
type SessionsCleanOutput struct {
	Cleaned int      `json:"cleaned"`
	Failed  int      `json:"failed"`
	IDs     []string `json:"ids"`
}

// SpriteSyncInput contains parameters for syncing files.
type SpriteSyncInput struct {
	SpriteName string `json:"sprite_name" jsonschema:"The name of the sprite to sync to"`
	SourceDir  string `json:"source_dir,omitempty" jsonschema:"Source directory to sync (default: current directory)"`
	BaseDir    string `json:"base_dir,omitempty" jsonschema:"Base directory on sprite (default: /handoffs)"`
}

// SpriteSyncOutput contains the result of a sync operation.
type SpriteSyncOutput struct {
	HandoffID string `json:"handoff_id"`
	TargetDir string `json:"target_dir"`
	Synced    int    `json:"synced"`
	Skipped   int    `json:"skipped"`
	Failed    int    `json:"failed"`
	Total     int    `json:"total"`
}

// SpriteReadInput contains parameters for reading a file from a sprite.
type SpriteReadInput struct {
	SpriteName string `json:"sprite_name" jsonschema:"The name of the sprite"`
	Path       string `json:"path" jsonschema:"Path to the file on the sprite"`
}

// SpriteReadOutput contains the file content.
type SpriteReadOutput struct {
	Content string `json:"content"`
	Path    string `json:"path"`
}

// SpriteWriteInput contains parameters for writing a file to a sprite.
type SpriteWriteInput struct {
	SpriteName string `json:"sprite_name" jsonschema:"The name of the sprite"`
	Path       string `json:"path" jsonschema:"Path where to write the file on the sprite"`
	Content    string `json:"content" jsonschema:"Content to write to the file"`
}

// SpriteWriteOutput contains the result of a write operation.
type SpriteWriteOutput struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
}

// SpriteListInput contains parameters for listing a directory on a sprite.
type SpriteListInput struct {
	SpriteName string `json:"sprite_name" jsonschema:"The name of the sprite"`
	Path       string `json:"path,omitempty" jsonschema:"Path to list (default: current directory)"`
}

// FileEntry represents a file or directory in a listing.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

// SpriteListOutput contains the directory listing.
type SpriteListOutput struct {
	Entries []FileEntry `json:"entries"`
	Path    string      `json:"path"`
	Count   int         `json:"count"`
}

// SpriteDeleteInput contains parameters for deleting a file or directory.
type SpriteDeleteInput struct {
	SpriteName string `json:"sprite_name" jsonschema:"The name of the sprite"`
	Path       string `json:"path" jsonschema:"Path to delete on the sprite"`
	Recursive  bool   `json:"recursive,omitempty" jsonschema:"Delete directories recursively"`
}

// SpriteDeleteOutput contains the result of a delete operation.
type SpriteDeleteOutput struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
}
