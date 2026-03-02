// Package config manages the global projector configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrNotFound is returned when the config file does not exist.
var ErrNotFound = errors.New("config file not found")

const (
	configDirName  = ".projector"
	configFileName = "projector-config.toml"

	// CurrentConfigVersion is the config schema version written by this binary.
	// Increment when a migration is needed; Load will reject files with a higher version.
	CurrentConfigVersion = 1
)

// ErrConfigVersionTooNew is returned when the config file was written by a newer version of projector.
var ErrConfigVersionTooNew = errors.New("config file was written by a newer version of projector; please upgrade")

// GlobalConfig is the top-level structure for ~/.projector/projector-config.toml.
type GlobalConfig struct {
	ConfigVersion  int                   `toml:"config-version"`
	ProjectsDir    string                `toml:"projects-dir"`
	RepoSearchDirs []string              `toml:"repo-search-dirs"`
	Editor         string                `toml:"editor,omitempty"`
	Repos          map[string]RepoConfig `toml:"repos"`
}

// RepoConfig holds per-repository overrides stored under [repos.<name>].
type RepoConfig struct {
	DefaultBase string `toml:"default-base"`
}

// ConfigDir returns the path to the projector config directory (~/.projector).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// ConfigFilePath returns the full path to projector-config.toml.
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads and parses the global config file.
// Returns ErrNotFound if the file does not exist.
func Load() (*GlobalConfig, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrNotFound
	}

	cfg := &GlobalConfig{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.ConfigVersion > CurrentConfigVersion {
		return nil, fmt.Errorf("%w (file version %d, supported version %d)",
			ErrConfigVersionTooNew, cfg.ConfigVersion, CurrentConfigVersion)
	}
	return cfg, nil
}

const editorComment = `# editor: command used by "pj project open". Accepts any executable that takes
# a directory path as its first positional argument (e.g. cursor, code, subl,
# bbedit, idea, zed, finder). You can specify a custom command or script here
# provided it follows the same convention.
`

// Save writes the config to disk, creating the config directory if needed.
// When the editor field is set, an explanatory comment is inserted above it.
func Save(cfg *GlobalConfig) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg.ConfigVersion = CurrentConfigVersion

	// Encode to a buffer so we can inject the editor comment before writing.
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	content := buf.String()
	if cfg.Editor != "" {
		content = strings.Replace(content, "editor = ", editorComment+"editor = ", 1)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ResolveBase determines the git base ref to use for a new worktree branch.
// Priority: per-repo default-base override → origin/main → origin/master → HEAD.
func ResolveBase(cfg *GlobalConfig, repoName, repoPath string) (string, error) {
	// 1. Per-repo override
	if repoCfg, ok := cfg.Repos[repoName]; ok && repoCfg.DefaultBase != "" {
		return repoCfg.DefaultBase, nil
	}

	// 2. origin/main
	if refExists(repoPath, "refs/remotes/origin/main") {
		return "origin/main", nil
	}

	// 3. HEAD (current branch of the default clone)
	return "HEAD", nil
}

// refExists checks whether the given git ref exists in repoPath.
// Uses os/exec directly to avoid a circular import with the git package.
func refExists(repoPath, ref string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	_ = out
	return err == nil
}

// Validate checks the config for required fields and returns a human-readable error.
func Validate(cfg *GlobalConfig) error {
	if strings.TrimSpace(cfg.ProjectsDir) == "" {
		return fmt.Errorf("projects-dir is required")
	}
	return nil
}
