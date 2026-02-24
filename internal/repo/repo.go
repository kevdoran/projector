// Package repo handles discovery and resolution of git repositories.
package repo

import (
	"fmt"
	"os"
	"path/filepath"
)

// Repo represents a git repository available to projector.
type Repo struct {
	// Name is the base directory name of the repository.
	Name string
	// Path is the absolute path to the repository root.
	Path string
}

// Discover searches the given directories (non-recursively) for git repositories.
// A directory is considered a git repository if it contains a .git subdirectory.
func Discover(searchDirs []string) ([]Repo, error) {
	var repos []Repo
	seen := map[string]bool{}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // skip missing dirs silently
			}
			return nil, fmt.Errorf("read dir %s: %w", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(dir, entry.Name())
			gitPath := filepath.Join(repoPath, ".git")
			info, err := os.Stat(gitPath)
			if err != nil {
				continue // not a git repo
			}
			if !info.IsDir() {
				continue // .git must be a directory (not a worktree .git file)
			}
			if seen[repoPath] {
				continue
			}
			seen[repoPath] = true
			repos = append(repos, Repo{Name: entry.Name(), Path: repoPath})
		}
	}

	return repos, nil
}

// ResolveRepos resolves a list of repository references (names or absolute paths)
// against the configured search directories.
// Names are matched against repositories discovered in searchDirs.
// Absolute paths are validated as git repositories directly.
func ResolveRepos(args []string, searchDirs []string) ([]Repo, error) {
	discovered, err := Discover(searchDirs)
	if err != nil {
		return nil, fmt.Errorf("discover repos: %w", err)
	}

	// Build a name → repo index for fast lookup
	byName := map[string]Repo{}
	for _, r := range discovered {
		byName[r.Name] = r
	}

	var result []Repo
	for _, arg := range args {
		if filepath.IsAbs(arg) {
			// Absolute path: validate it is a git repo
			gitPath := filepath.Join(arg, ".git")
			info, err := os.Stat(gitPath)
			if err != nil || !info.IsDir() {
				return nil, fmt.Errorf("%q is not a valid git repository (no .git directory)", arg)
			}
			result = append(result, Repo{Name: filepath.Base(arg), Path: arg})
		} else {
			// Name lookup
			r, ok := byName[arg]
			if !ok {
				return nil, fmt.Errorf("repository %q not found in search directories", arg)
			}
			result = append(result, r)
		}
	}

	return result, nil
}
