package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Config holds all user-configurable preferences.
type Config struct {
	// Panel settings
	Panel PanelConfig `toml:"panel"`

	// Editor settings
	Editor EditorConfig `toml:"editor"`

	// Keybindings
	Keybindings KeybindingConfig `toml:"keybindings"`

	// UI settings
	UI UIConfig `toml:"ui"`
}

// PanelConfig holds per-panel preferences.
type PanelConfig struct {
	// Sort settings (per-panel, but defaults here)
	SortBy    string `toml:"sort_by"`    // name, size, time, ext
	DirsFirst bool   `toml:"dirs_first"` // directories first
	Reverse   bool   `toml:"reverse"`    // reverse order

	// Filter settings
	IncludeMask string `toml:"include_mask"` // filepath.Match pattern

	// Hotlist (bookmarked directories)
	Hotlist []string `toml:"hotlist"`
}

// EditorConfig holds external editor settings.
type EditorConfig struct {
	// Command to launch the editor (e.g., "code --wait", "notepad++", "vim")
	// Empty means use EDITOR/VISUAL environment variables.
	Command string `toml:"command"`

	// Arguments to pass to the editor (optional)
	Args []string `toml:"args"`
}

// KeybindingConfig holds custom keybindings.
// Empty string means use default.
type KeybindingConfig struct {
	Copy         string `toml:"copy"`           // default F5
	Move         string `toml:"move"`           // default F6
	Rename       string `toml:"rename"`         // default Shift+F6
	Delete       string `toml:"delete"`         // default F8
	View         string `toml:"view"`           // default F3
	Edit         string `toml:"edit"`           // default F4
	CopyFile     string `toml:"copy_file"`      // default Shift+F5
	MoveFile     string `toml:"move_file"`      // default F6
	MakeDir      string `toml:"make_dir"`       // default F7
	DeleteFile   string `toml:"delete_file"`    // default F8
	QuickView    string `toml:"quick_view"`     // default Ctrl+Q
	MultiRename  string `toml:"multi_rename"`   // default Ctrl+M
	QuickSearch  string `toml:"quick_search"`   // default letter
	DriveLeft    string `toml:"drive_left"`     // default Alt+F1
	DriveRight   string `toml:"drive_right"`    // default Alt+F2
	ParentDir    string `toml:"parent_dir"`     // default Backspace
	Quit         string `toml:"quit"`           // default F10
	ToggleSelect string `toml:"toggle_select"`  // default Insert
	SelectAll    string `toml:"select_all"`     // default *
	InvertSelect string `toml:"invert_select"`  // default *
	DeselectAll  string `toml:"deselect_all"`   // default -
	ParentDirAlt string `toml:"parent_dir_alt"` // default Ctrl+PgUp
	PageUp       string `toml:"page_up"`        // default PgUp
	PageDown     string `toml:"page_down"`      // default PgDown
	Home         string `toml:"home"`           // default Home
	End          string `toml:"end"`            // default End
	Tab          string `toml:"tab"`            // default Tab
	Enter        string `toml:"enter"`          // default Enter
	Escape       string `toml:"escape"`         // default Escape
	F1           string `toml:"f1"`             // default F1
	F2           string `toml:"f2"`             // default F2
	F3           string `toml:"f3"`             // default F3
	F4           string `toml:"f4"`             // default F4
	F5           string `toml:"f5"`             // default F5
	F6           string `toml:"f6"`             // default F6
	F7           string `toml:"f7"`             // default F7
	F8           string `toml:"f8"`             // default F8
	F9           string `toml:"f9"`             // default F9
	F10          string `toml:"f10"`            // default F10
}

// UIConfig holds UI appearance settings.
type UIConfig struct {
	// Show hidden files
	ShowHidden bool `toml:"show_hidden"`

	// Show file extensions
	ShowExtensions bool `toml:"show_extensions"`

	// Wrap long names in panel
	WrapNames bool `toml:"wrap_names"`

	// Show file sizes in human-readable format
	HumanReadableSizes bool `toml:"human_readable_sizes"`

	// Show modification time
	ShowModTime bool `toml:"show_mod_time"`

	// Theme (future: dark, light, custom)
	Theme string `toml:"theme"`

	// Language (future: i18n)
	Language string `toml:"language"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Panel: PanelConfig{
			SortBy:    "name",
			DirsFirst: true,
			Reverse:   false,
			Hotlist:   []string{},
		},
		Editor: EditorConfig{
			Command: "",
			Args:    []string{},
		},
		Keybindings: KeybindingConfig{},
		UI: UIConfig{
			ShowHidden:         false,
			ShowExtensions:     true,
			WrapNames:          false,
			HumanReadableSizes: true,
			ShowModTime:        true,
			Theme:              "default",
			Language:           "en",
		},
	}
}

// ConfigPath returns the path to the config file.
func ConfigPath() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	default:
		baseDir = os.Getenv("XDG_CONFIG_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".config")
		}
	}

	configDir := filepath.Join(baseDir, "tui-commander")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.toml"), nil
}

// LoadConfig loads the configuration from the config file.
// If the file doesn't exist, returns the default config.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parse config: %w", err)
	}

	// Merge with defaults for any missing fields
	cfg = mergeWithDefaults(cfg)

	return cfg, nil
}

// SaveConfig saves the configuration to the config file.
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// mergeWithDefaults merges a config with defaults for any zero values.
func mergeWithDefaults(cfg Config) Config {
	def := DefaultConfig()

	if cfg.Panel.SortBy == "" {
		cfg.Panel.SortBy = def.Panel.SortBy
	}
	if cfg.Editor.Command == "" && cfg.Editor.Args == nil {
		cfg.Editor.Args = def.Editor.Args
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = def.UI.Theme
	}
	if cfg.UI.Language == "" {
		cfg.UI.Language = def.UI.Language
	}
	// Keybindings: empty strings are intentional (use defaults)
	// UI: false values are intentional

	return cfg
}
