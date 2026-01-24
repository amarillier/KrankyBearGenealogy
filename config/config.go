package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	
	"fyne.io/fyne/v2"
)

// Config stores application configuration.
type Config struct {
	LastDatabase      string            `json:"last_database"`
	RecentDatabases   []string          `json:"recent_databases"`
	LastPersonID      map[string]int64  `json:"last_person_id"`   // map of database path -> last selected person ID
	FocusUserID       map[string]int64  `json:"focus_user_id"`    // map of database path -> focus user ID
	OpenWithFocusUser bool              `json:"open_with_focus_user"` // if true, open with focus user; if false, open with last person
	KeyboardShortcuts map[string]string `json:"keyboard_shortcuts"` // map of action -> key name (e.g., "AddPerson" -> "N")
}

// Load loads the configuration from the config file.
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return &Config{
			RecentDatabases:   []string{},
			LastPersonID:      make(map[string]int64),
			FocusUserID:       make(map[string]int64),
			KeyboardShortcuts: make(map[string]string),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist yet, return default
		return &Config{
			RecentDatabases:   []string{},
			LastPersonID:      make(map[string]int64),
			FocusUserID:       make(map[string]int64),
			KeyboardShortcuts: make(map[string]string),
		}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{
			RecentDatabases:   []string{},
			LastPersonID:      make(map[string]int64),
			FocusUserID:       make(map[string]int64),
			KeyboardShortcuts: make(map[string]string),
		}, nil
	}
	
	// Ensure maps are initialized
	if cfg.LastPersonID == nil {
		cfg.LastPersonID = make(map[string]int64)
	}
	if cfg.FocusUserID == nil {
		cfg.FocusUserID = make(map[string]int64)
	}
	if cfg.KeyboardShortcuts == nil {
		cfg.KeyboardShortcuts = make(map[string]string)
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

// ClearRecentDatabases clears the recent databases list.
func (c *Config) ClearRecentDatabases() {
	c.RecentDatabases = []string{}
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

// DefaultKeyboardShortcuts returns the default keyboard shortcut mappings.
func DefaultKeyboardShortcuts() map[string]string {
	return map[string]string{
		"AddPerson":            "N",
		"DeletePerson":         "D",
		"EditPerson":           "E",
		"FocusSearch":          "F",
		"GoToFocusPerson":      "G",
		"OpenDatabase":         "O",
		"BackupDatabase":       "B",      // Backup database
		"DatabaseMaintenance":  "L",      // Database maintenance (L for cLean/maintenance)
		"ToggleBookmark":       "W",      // Bookmark/unmark person (W for "watch list")
		"MediaLibrary":         "M",      // Media library
		"Settings":             "S",      // Primary
		"SettingsAlt":          "Comma",  // Alternative (Mac standard)
		"KeyboardShortcuts":    "K",      // Open keyboard shortcuts dialog
		"About":                "I",      // About/Info dialog
		"CheckUpdate":          "U",      // Check for updates
		"Help":                 "Slash",  // Help (Cmd+/)
		"Quit":                 "Q",
		"Statistics":           "T",
		"DataQuality":          "R",
		"AdvancedSearch":       "ShiftF", // Shift+F for advanced search
		"SwitchToFamily":       "1",
		"SwitchToPedigree":     "2",
		"SwitchToIndividual":   "3",
	}
}

// GetShortcut returns the key for a given action, or the default if not customized.
func (c *Config) GetShortcut(action string) string {
	if c.KeyboardShortcuts == nil {
		c.KeyboardShortcuts = make(map[string]string)
	}
	
	if key, ok := c.KeyboardShortcuts[action]; ok {
		return key
	}
	
	// Return default
	defaults := DefaultKeyboardShortcuts()
	return defaults[action]
}

// SetShortcut sets a custom shortcut for an action.
func (c *Config) SetShortcut(action, key string) {
	if c.KeyboardShortcuts == nil {
		c.KeyboardShortcuts = make(map[string]string)
	}
	c.KeyboardShortcuts[action] = key
}

// ResetShortcutsToDefaults resets all shortcuts to their default values.
func (c *Config) ResetShortcutsToDefaults() {
	c.KeyboardShortcuts = DefaultKeyboardShortcuts()
}

// HasShiftModifier returns true if the shortcut string includes Shift modifier
func HasShiftModifier(s string) bool {
	return strings.HasPrefix(strings.ToUpper(s), "SHIFT")
}

// StringToKeyName converts a string to a fyne.KeyName.
func StringToKeyName(s string) fyne.KeyName {
	// Convert to uppercase for consistency
	s = strings.ToUpper(s)
	
	// Handle Shift+Key combinations by stripping "SHIFT" prefix
	if strings.HasPrefix(s, "SHIFT") {
		s = strings.TrimPrefix(s, "SHIFT")
	}
	
	// Letters A-Z
	switch s {
	case "A": return fyne.KeyA
	case "B": return fyne.KeyB
	case "C": return fyne.KeyC
	case "D": return fyne.KeyD
	case "E": return fyne.KeyE
	case "F": return fyne.KeyF
	case "G": return fyne.KeyG
	case "H": return fyne.KeyH
	case "I": return fyne.KeyI
	case "J": return fyne.KeyJ
	case "K": return fyne.KeyK
	case "L": return fyne.KeyL
	case "M": return fyne.KeyM
	case "N": return fyne.KeyN
	case "O": return fyne.KeyO
	case "P": return fyne.KeyP
	case "Q": return fyne.KeyQ
	case "R": return fyne.KeyR
	case "S": return fyne.KeyS
	case "T": return fyne.KeyT
	case "U": return fyne.KeyU
	case "V": return fyne.KeyV
	case "W": return fyne.KeyW
	case "X": return fyne.KeyX
	case "Y": return fyne.KeyY
	case "Z": return fyne.KeyZ
	// Numbers 0-9
	case "0": return fyne.Key0
	case "1": return fyne.Key1
	case "2": return fyne.Key2
	case "3": return fyne.Key3
	case "4": return fyne.Key4
	case "5": return fyne.Key5
	case "6": return fyne.Key6
	case "7": return fyne.Key7
	case "8": return fyne.Key8
	case "9": return fyne.Key9
	// Special keys
	case "COMMA": return fyne.KeyComma
	case "PERIOD": return fyne.KeyPeriod
	case "SLASH": return fyne.KeySlash
	case "BACKSLASH": return fyne.KeyBackslash
	case "SEMICOLON": return fyne.KeySemicolon
	case "MINUS": return fyne.KeyMinus
	case "EQUAL": return fyne.KeyEqual
	default: return fyne.KeyUnknown
	}
}

// KeyNameToString converts a fyne.KeyName to a string.
func KeyNameToString(key fyne.KeyName) string {
	switch key {
	case fyne.KeyA: return "A"
	case fyne.KeyB: return "B"
	case fyne.KeyC: return "C"
	case fyne.KeyD: return "D"
	case fyne.KeyE: return "E"
	case fyne.KeyF: return "F"
	case fyne.KeyG: return "G"
	case fyne.KeyH: return "H"
	case fyne.KeyI: return "I"
	case fyne.KeyJ: return "J"
	case fyne.KeyK: return "K"
	case fyne.KeyL: return "L"
	case fyne.KeyM: return "M"
	case fyne.KeyN: return "N"
	case fyne.KeyO: return "O"
	case fyne.KeyP: return "P"
	case fyne.KeyQ: return "Q"
	case fyne.KeyR: return "R"
	case fyne.KeyS: return "S"
	case fyne.KeyT: return "T"
	case fyne.KeyU: return "U"
	case fyne.KeyV: return "V"
	case fyne.KeyW: return "W"
	case fyne.KeyX: return "X"
	case fyne.KeyY: return "Y"
	case fyne.KeyZ: return "Z"
	case fyne.Key0: return "0"
	case fyne.Key1: return "1"
	case fyne.Key2: return "2"
	case fyne.Key3: return "3"
	case fyne.Key4: return "4"
	case fyne.Key5: return "5"
	case fyne.Key6: return "6"
	case fyne.Key7: return "7"
	case fyne.Key8: return "8"
	case fyne.Key9: return "9"
	case fyne.KeyComma: return "Comma"
	case fyne.KeyPeriod: return "Period"
	case fyne.KeySlash: return "Slash"
	case fyne.KeyBackslash: return "Backslash"
	case fyne.KeySemicolon: return "Semicolon"
	case fyne.KeyMinus: return "Minus"
	case fyne.KeyEqual: return "Equal"
	default: return "Unknown"
	}
}
