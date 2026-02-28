// Package project manages per-project configuration and worktree discovery.
package project

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ErrNotFound is returned when a project config file is not found.
var ErrNotFound = errors.New("project config not found")

// ErrConfigVersionTooNew is returned when the project config was written by a newer version of projector.
var ErrConfigVersionTooNew = errors.New("project config was written by a newer version of projector; please upgrade")

const (
	projectFileName = ".projector.toml"

	// CurrentConfigVersion is the project config schema version written by this binary.
	CurrentConfigVersion = 1
)

// Status represents the lifecycle state of a project.
type Status string

const (
	StatusActive        Status = "active"
	StatusArchived      Status = "archived"
	StatusArchiveFailed Status = "archive-failed"
)

// ProjectConfig is the structure of <project-dir>/.projector.toml.
type ProjectConfig struct {
	ConfigVersion     int              `toml:"config-version"`
	Project           ProjectMeta      `toml:"project"`
	ArchivedWorktrees []WorktreeRecord `toml:"archived-worktrees"`
}

// ProjectMeta holds the core metadata stored in .projector.toml.
type ProjectMeta struct {
	Name       string     `toml:"name"`
	CreatedAt  time.Time  `toml:"created-at"`
	Status     Status     `toml:"status"`
	ArchivedAt *time.Time `toml:"archived-at,omitempty"`
}

// WorktreeRecord captures worktree state at archive time for future restore.
type WorktreeRecord struct {
	RepoName     string `toml:"repo-name"`
	RepoPath     string `toml:"repo-path"`
	WorktreePath string `toml:"worktree-path"`
	Branch       string `toml:"branch"`
	Commit       string `toml:"commit,omitempty"`
}

// WorktreeInfo is the runtime (dynamically discovered) view of a live worktree.
type WorktreeInfo struct {
	RepoName     string
	RepoPath     string
	WorktreePath string
	Branch       string
}

// ProjectDir returns the expected directory for a project.
func ProjectDir(projectsDir, name string) string {
	return filepath.Join(projectsDir, name)
}

// Load reads and parses the .projector.toml in the given project directory.
func Load(projectDir string) (*ProjectConfig, error) {
	path := filepath.Join(projectDir, projectFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	cfg := &ProjectConfig{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse project config %s: %w", path, err)
	}
	if cfg.ConfigVersion > CurrentConfigVersion {
		return nil, fmt.Errorf("%w (file version %d, supported version %d)",
			ErrConfigVersionTooNew, cfg.ConfigVersion, CurrentConfigVersion)
	}
	return cfg, nil
}

// Save writes the ProjectConfig to <projectDir>/.projector.toml.
func Save(cfg *ProjectConfig, projectDir string) error {
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	path := filepath.Join(projectDir, projectFileName)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create project file: %w", err)
	}
	defer f.Close()
	cfg.ConfigVersion = CurrentConfigVersion
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode project config: %w", err)
	}
	return nil
}

// ListAll scans projectsDir for subdirectories containing .projector.toml.
// Results are sorted: active projects (alpha) before archived projects (alpha).
// Dirs without a valid .projector.toml are silently skipped.
func ListAll(projectsDir string) ([]*ProjectConfig, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var active, archived []*ProjectConfig
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, entry.Name())
		cfg, err := Load(dir)
		if err != nil {
			continue // skip invalid dirs
		}
		switch cfg.Project.Status {
		case StatusActive:
			active = append(active, cfg)
		default:
			archived = append(archived, cfg)
		}
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Project.Name < active[j].Project.Name
	})
	sort.Slice(archived, func(i, j int) bool {
		return archived[i].Project.Name < archived[j].Project.Name
	})

	return append(active, archived...), nil
}

// FindProjectDir walks up from startDir looking for a directory containing
// .projector.toml. Returns the directory path if found, or ErrNotFound.
func FindProjectDir(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, projectFileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return "", ErrNotFound
}

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateName returns an error if name is not a valid project name.
// Names must be non-empty and contain only alphanumeric characters, hyphens,
// underscores, and dots — no spaces or slashes.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("project name %q is invalid: only letters, digits, hyphens, underscores, and dots are allowed", name)
	}
	return nil
}

// DiscoverWorktrees scans projectDir for subdirectories that are git worktrees.
// A worktree directory contains a .git *file* (not directory) with a gitdir line.
func DiscoverWorktrees(projectDir string) ([]WorktreeInfo, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("read project dir: %w", err)
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worktreePath := filepath.Join(projectDir, entry.Name())
		gitFilePath := filepath.Join(worktreePath, ".git")

		info, err := os.Stat(gitFilePath)
		if err != nil || info.IsDir() {
			continue // not a worktree (no .git file or it's a dir)
		}

		// Read the .git file to extract the gitdir path
		content, err := os.ReadFile(gitFilePath)
		if err != nil {
			continue
		}
		gitdirLine := strings.TrimSpace(string(content))
		if !strings.HasPrefix(gitdirLine, "gitdir:") {
			continue
		}
		gitdirPath := strings.TrimSpace(strings.TrimPrefix(gitdirLine, "gitdir:"))

		// gitdirPath: /path/to/repo/.git/worktrees/<name>
		repoPath := extractRepoPath(gitdirPath)
		if repoPath == "" {
			continue
		}

		branch := gitCurrentBranch(worktreePath)

		worktrees = append(worktrees, WorktreeInfo{
			RepoName:     filepath.Base(repoPath),
			RepoPath:     repoPath,
			WorktreePath: worktreePath,
			Branch:       branch,
		})
	}

	return worktrees, nil
}

// extractRepoPath derives the main repo path from a worktree gitdir path.
// Input:  /path/to/repo/.git/worktrees/<name>
// Output: /path/to/repo
func extractRepoPath(gitdirPath string) string {
	gitdirPath = filepath.Clean(gitdirPath)
	// Strip /<worktreeName>  → .../repo/.git/worktrees
	worktreesDir := filepath.Dir(gitdirPath)
	// Strip /worktrees       → .../repo/.git
	dotGitDir := filepath.Dir(worktreesDir)
	// Strip /.git            → .../repo
	repoDir := filepath.Dir(dotGitDir)

	if filepath.Base(worktreesDir) != "worktrees" {
		return ""
	}
	if filepath.Base(dotGitDir) != ".git" {
		return ""
	}
	return repoDir
}

// gitCurrentBranch returns the current branch name in dir using os/exec.
// Returns empty string on any error (e.g. detached HEAD).
func gitCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "" // detached HEAD
	}
	return branch
}
