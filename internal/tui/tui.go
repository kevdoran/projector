// Package tui provides interactive terminal UI helpers using charmbracelet/huh.
package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/repo"
)

// ErrAborted is returned when the user presses ESC to abort an interactive prompt.
var ErrAborted = errors.New("aborted")

// SelectRepos presents an interactive multi-select form for choosing repositories.
// available is the full list of discovered repos; exclude contains repo names to omit.
func SelectRepos(available []repo.Repo, exclude []string) ([]repo.Repo, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("no repositories available to select")
	}

	excludeSet := map[string]bool{}
	for _, name := range exclude {
		excludeSet[name] = true
	}

	var options []huh.Option[string]
	repoByName := map[string]repo.Repo{}
	for _, r := range available {
		if excludeSet[r.Name] {
			continue
		}
		options = append(options, huh.NewOption(r.Name+" ("+r.Path+")", r.Name))
		repoByName[r.Name] = r
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no repositories available after applying exclusions")
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select repositories to include in this project").
				Description("Use space to toggle, enter to confirm, esc to abort").
				Options(options...).
				Value(&selected),
		),
	).WithKeyMap(km)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrAborted
		}
		return nil, fmt.Errorf("repo selection: %w", err)
	}

	var result []repo.Repo
	for _, name := range selected {
		if r, ok := repoByName[name]; ok {
			result = append(result, r)
		}
	}
	return result, nil
}

// InitConfig runs an interactive first-time setup form and returns a populated GlobalConfig.
func InitConfig() (*config.GlobalConfig, error) {
	var projectsDir string
	var repoSearchDirsRaw string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Projects directory").
				Description("Absolute path where project directories will be created (e.g. ~/projects)").
				Value(&projectsDir).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("projects directory is required")
					}
					return nil
				}),

			huh.NewInput().
				Title("Repository search directories (optional)").
				Description("Comma-separated list of directories to search for git repositories").
				Value(&repoSearchDirsRaw),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("first-time setup: %w", err)
	}

	cfg := &config.GlobalConfig{
		ProjectsDir: expandHome(projectsDir),
	}

	if repoSearchDirsRaw != "" {
		for _, d := range strings.Split(repoSearchDirsRaw, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				cfg.RepoSearchDirs = append(cfg.RepoSearchDirs, expandHome(d))
			}
		}
	}

	return cfg, nil
}

// expandHome replaces a leading ~/ (or bare ~) with the user's home directory.
// Paths like ~word are left unchanged to avoid silent corruption.
func expandHome(path string) string {
	path = strings.TrimSpace(path)
	switch {
	case path == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	case strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	default:
		return path
	}
}
