package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config stores application configuration.
type Config struct {
	LastDatabase      string            `json:"last_database"`
	RecentDatabases   []string          `json:"recent_databases"`
	LastPersonID      map[string]int64  `json:"last_person_id"`   // map of database path -> last selected person ID
	FocusUserID       map[string]int64  `json:"focus_user_id"`    // map of database path -> focus user ID
	OpenWithFocusUser bool              `json:"open_with_focus_user"` // if true, open with focus user; if false, open with last person
}

// Load loads the configuration from the config file.
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return &Config{
			RecentDatabases: []string{},
			LastPersonID:    make(map[string]int64),
			FocusUserID:     make(map[string]int64),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist yet, return default
		return &Config{
			RecentDatabases: []string{},
			LastPersonID:    make(map[string]int64),
			FocusUserID:     make(map[string]int64),
		}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{
			RecentDatabases: []string{},
			LastPersonID:    make(map[string]int64),
			FocusUserID:     make(map[string]int64),
		}, nil
	}
	
	// Ensure maps are initialized
	if cfg.LastPersonID == nil {
		cfg.LastPersonID = make(map[string]int64)
	}
	if cfg.FocusUserID == nil {
		cfg.FocusUserID = make(map[string]int64)
	}

	return &cfg, nil
}

// Save saves the configuration to the config file.
func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// AddRecentDatabase adds a database to the recent list.
func (c *Config) AddRecentDatabase(path string) {
	// Remove if already exists
	for i, db := range c.RecentDatabases {
		if db == path {
			c.RecentDatabases = append(c.RecentDatabases[:i], c.RecentDatabases[i+1:]...)
			break
		}
	}

	// Add to front
	c.RecentDatabases = append([]string{path}, c.RecentDatabases...)

	// Keep only last 10
	if len(c.RecentDatabases) > 10 {
		c.RecentDatabases = c.RecentDatabases[:10]
	}

	c.LastDatabase = path
}

// SetLastPersonForDatabase sets the last selected person for a specific database.
func (c *Config) SetLastPersonForDatabase(dbPath string, personID int64) {
	if c.LastPersonID == nil {
		c.LastPersonID = make(map[string]int64)
	}
	c.LastPersonID[dbPath] = personID
}

// GetLastPersonForDatabase gets the last selected person for a specific database.
func (c *Config) GetLastPersonForDatabase(dbPath string) int64 {
	if c.LastPersonID == nil {
		return 0
	}
	return c.LastPersonID[dbPath]
}

// SetFocusUserForDatabase sets the focus user for a specific database.
func (c *Config) SetFocusUserForDatabase(dbPath string, personID int64) {
	if c.FocusUserID == nil {
		c.FocusUserID = make(map[string]int64)
	}
	c.FocusUserID[dbPath] = personID
}

// GetFocusUserForDatabase gets the focus user for a specific database.
func (c *Config) GetFocusUserForDatabase(dbPath string) int64 {
	if c.FocusUserID == nil {
		return 0
	}
	return c.FocusUserID[dbPath]
}

// getConfigPath returns the path to the config file.
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".krankybear-genealogy", "config.json"), nil
}
