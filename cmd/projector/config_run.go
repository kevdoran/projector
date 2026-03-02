package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/repo"
	"github.com/kevdoran/projector/internal/tui"
)

func newConfigSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run interactive configuration wizard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigWizard()
		},
	}
}

func runConfigWizard() error {
	// Load existing config to pre-populate fields.
	existing, err := config.Load()
	if err != nil && err != config.ErrNotFound {
		return fmt.Errorf("load config: %w", err)
	}

	var projectsDir string
	var repoSearchDirsRaw string

	if existing != nil {
		projectsDir = existing.ProjectsDir
		repoSearchDirsRaw = strings.Join(existing.RepoSearchDirs, ", ")
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Projects directory").
				Description("Where project directories will be created (e.g. ~/projects).\nThe path will be created if it doesn't exist.").
				Value(&projectsDir).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("projects directory is required")
					}
					return nil
				}),

			huh.NewInput().
				Title("Repository search directories").
				Description("Parent directories of your git repository clones.\nOnly direct subdirectories are searched (not recursive).\nMultiple directories separated by commas.").
				Value(&repoSearchDirsRaw).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("at least one repository search directory is required")
					}
					return nil
				}),
		),
	).WithKeyMap(km)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("Aborted.")
			return nil
		}
		return fmt.Errorf("config wizard: %w", err)
	}

	cfg := &config.GlobalConfig{
		ProjectsDir: tui.ExpandHome(projectsDir),
	}

	// Preserve existing fields not covered by the wizard.
	if existing != nil {
		cfg.DefaultEditor = existing.DefaultEditor
		cfg.Editors = existing.Editors
		cfg.Repos = existing.Repos
	}

	if repoSearchDirsRaw != "" {
		for _, d := range strings.Split(repoSearchDirsRaw, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				cfg.RepoSearchDirs = append(cfg.RepoSearchDirs, tui.ExpandHome(d))
			}
		}
	}

	// Validate and show results.
	var warnings []string

	projectsDirExpanded := cfg.ProjectsDir
	if info, err := os.Stat(projectsDirExpanded); err != nil {
		if os.IsNotExist(err) {
			parent := projectsDirExpanded
			for parent != "/" && parent != "." {
				parent = strings.TrimRight(parent, "/")
				idx := strings.LastIndex(parent, "/")
				if idx < 0 {
					break
				}
				parent = parent[:idx]
				if parent == "" {
					parent = "/"
				}
				if _, err := os.Stat(parent); err == nil {
					break
				}
			}
			fmt.Printf("  projects-dir: %s (will be created)\n", projectsDirExpanded)
		} else {
			warnings = append(warnings, fmt.Sprintf("projects-dir: cannot access %s: %v", projectsDirExpanded, err))
			fmt.Printf("  projects-dir: %s (warning: %v)\n", projectsDirExpanded, err)
		}
	} else if !info.IsDir() {
		warnings = append(warnings, fmt.Sprintf("projects-dir: %s exists but is not a directory", projectsDirExpanded))
		fmt.Printf("  projects-dir: %s (warning: not a directory)\n", projectsDirExpanded)
	} else {
		fmt.Printf("  projects-dir: %s\n", projectsDirExpanded)
	}

	for _, dir := range cfg.RepoSearchDirs {
		repos, err := repo.Discover([]string{dir})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("repo-search-dir %s: %v", dir, err))
			fmt.Printf("  repo-search-dir: %s (warning: %v)\n", dir, err)
		} else if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			warnings = append(warnings, fmt.Sprintf("repo-search-dir %s: directory does not exist", dir))
			fmt.Printf("  repo-search-dir: %s (warning: does not exist)\n", dir)
		} else if len(repos) == 0 {
			warnings = append(warnings, fmt.Sprintf("repo-search-dir %s: no git repositories found", dir))
			fmt.Printf("  repo-search-dir: %s (warning: no git repositories found)\n", dir)
		} else {
			fmt.Printf("  repo-search-dir: %s (%d repos found)\n", dir, len(repos))
		}
	}

	if len(warnings) > 0 {
		var choice string
		selectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("There are warnings. What would you like to do?").
					Options(
						huh.NewOption("Save anyway", "save"),
						huh.NewOption("Edit", "edit"),
						huh.NewOption("Abort", "abort"),
					).
					Value(&choice),
			),
		).WithKeyMap(km)

		if err := selectForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println("Aborted.")
				return nil
			}
			return fmt.Errorf("config wizard: %w", err)
		}

		switch choice {
		case "edit":
			return runConfigWizard()
		case "abort":
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	path, _ := config.ConfigFilePath()
	fmt.Printf("Configuration saved to %s\n", path)
	return nil
}
