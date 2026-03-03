package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
