package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

// resolveProject resolves a project from optional positional args or the current directory.
// If args contains a project name, that name is used.
// Otherwise the current working directory is walked upward to find a .projector.toml.
func resolveProject(projectsDir string, args []string) (string, *project.ProjectConfig, error) {
	var projectDir string

	if len(args) > 0 {
		projectDir = project.ProjectDir(projectsDir, args[0])
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", nil, fmt.Errorf("get cwd: %w", err)
		}
		dir, err := project.FindProjectDir(cwd)
		if err != nil {
			return "", nil, fmt.Errorf("could not detect project from current directory: %w", err)
		}
		projectDir = dir
	}

	p, err := project.Load(projectDir)
	if err != nil {
		return "", nil, fmt.Errorf("load project at %s: %w", projectDir, err)
	}

	return projectDir, p, nil
}

// fetchIfRemote fetches the underlying ref when repoBase is a remote-tracking
// ref (e.g. "origin/main") so it is up to date before use. No-op for local refs.
func fetchIfRemote(repoName, repoPath, repoBase string) error {
	remote, err := git.RemoteForRef(repoPath, repoBase)
	if err != nil {
		return fmt.Errorf("check remote for %s: %w", repoName, err)
	}
	if remote != "" {
		ref := strings.TrimPrefix(repoBase, remote+"/")
		fmt.Printf("  fetching %s in %s…\n", repoBase, repoName)
		if err := git.FetchRef(repoPath, remote, ref); err != nil {
			return fmt.Errorf("fetch %s in %s: %w", repoBase, repoName, err)
		}
	}
	return nil
}

// resolveBaseRef resolves a user-supplied --base ref against a single repo.
//
// Resolution order:
//  1. If the ref names a remote explicitly (e.g. "origin/feature"), it is
//     fetched and verified as-is.
//  2. Otherwise, if a matching local ref exists, it is used as-is.
//  3. Otherwise, when the ref is unqualified and not found locally, it is
//     retried against the repo's default remote — fetching "<remote>/<base>"
//     so a bare ref like "feature" resolves to "origin/feature".
//
// It returns the resolved ref (possibly rewritten to "<remote>/<base>") and
// whether the ref was found.
func resolveBaseRef(repoName, repoPath, base string) (resolved string, found bool, err error) {
	// Fetch first if the ref explicitly names a remote, so it is up to date.
	if err := fetchIfRemote(repoName, repoPath, base); err != nil {
		return "", false, err
	}

	exists, err := git.RefExists(repoPath, base)
	if err != nil {
		return "", false, fmt.Errorf("check ref for %s: %w", repoName, err)
	}
	if exists {
		return base, true, nil
	}

	// The ref was not found. If it already named a remote, there is nothing
	// more to try.
	remote, err := git.RemoteForRef(repoPath, base)
	if err != nil {
		return "", false, fmt.Errorf("check remote for %s: %w", repoName, err)
	}
	if remote != "" {
		return base, false, nil
	}

	// Unqualified ref not found locally — retry against the default remote so
	// the user does not have to spell out "origin/".
	defRemote, err := git.DefaultRemote(repoPath)
	if err != nil {
		return "", false, fmt.Errorf("default remote for %s: %w", repoName, err)
	}
	if defRemote == "" {
		return base, false, nil
	}

	candidate := defRemote + "/" + base
	fmt.Printf("  fetching %s in %s…\n", candidate, repoName)
	if err := git.FetchRef(repoPath, defRemote, base); err != nil {
		// A fetch failure means the ref does not exist on the remote.
		return base, false, nil
	}
	exists, err = git.RefExists(repoPath, candidate)
	if err != nil {
		return "", false, fmt.Errorf("check ref for %s: %w", repoName, err)
	}
	if exists {
		return candidate, true, nil
	}
	return base, false, nil
}

// projectNameFromWorktreePath extracts a pj project name from a worktree path
// if the path is under projectsDir. Returns "-" if the path is not under projectsDir.
func projectNameFromWorktreePath(projectsDir, worktreePath string) string {
	rel, err := filepath.Rel(projectsDir, worktreePath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "-"
	}
	// The first path component is the project name.
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) == 0 || parts[0] == "" {
		return "-"
	}
	return parts[0]
}

// printNoReposFound prints a helpful message when no git repositories are
// found in the configured search directories, and directs the user to
// pj config setup to fix the issue.
func printNoReposFound(searchDirs []string) {
	fmt.Println("No git repositories found.")
	if len(searchDirs) == 0 {
		fmt.Println("No repository search directories are configured.")
	} else {
		fmt.Println("Searched directories:")
		for _, dir := range searchDirs {
			fmt.Printf("  %s\n", dir)
		}
	}
	fmt.Println("\nRun 'pj config setup' to update your repository search directories.")
}
