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

// EditorOption represents a selectable editor in the SelectEditor prompt.
type EditorOption struct {
	Name     string // display name, e.g. "Cursor"
	Command  string // value stored in config, e.g. "cursor"
	Terminal bool   // if true, print cd+command instead of launching
}

// SelectEditor presents an interactive single-select prompt for choosing an editor.
// Only editors that are available should be passed in (no install annotations).
func SelectEditor(options []EditorOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no editors available")
	}

	var huhOptions []huh.Option[string]
	for _, o := range options {
		huhOptions = append(huhOptions, huh.NewOption(o.Name, o.Command))
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose an editor").
				Description("Select with arrow keys, confirm with enter, esc to abort").
				Options(huhOptions...).
				Value(&selected),
		),
	).WithKeyMap(km)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", fmt.Errorf("editor selection: %w", err)
	}
	return selected, nil
}

// ExpandHome replaces a leading ~/ (or bare ~) with the user's home directory.
// Paths like ~word are left unchanged to avoid silent corruption.
func ExpandHome(path string) string {
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
