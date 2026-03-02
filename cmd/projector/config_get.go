package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
)

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Long: `Get an individual configuration value.

Supported keys:
  projects-dir                   Path where project directories are created
  repo-search-dirs               Comma-separated list of repo search directories
  editor                         Editor command for 'pj project open'
  repos.<name>.default-base      Per-repo default branch base`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			cfg, err := config.Load()
			if err != nil {
				if err == config.ErrNotFound {
					return fmt.Errorf("no configuration found; run 'pj config setup' to set up pj")
				}
				return fmt.Errorf("load config: %w", err)
			}

			switch {
			case key == "projects-dir":
				fmt.Println(cfg.ProjectsDir)

			case key == "repo-search-dirs":
				fmt.Println(strings.Join(cfg.RepoSearchDirs, ","))

			case key == "editor":
				fmt.Println(cfg.Editor)

			case strings.HasPrefix(key, "repos.") && strings.HasSuffix(key, ".default-base"):
				trimmed := strings.TrimPrefix(key, "repos.")
				trimmed = strings.TrimSuffix(trimmed, ".default-base")
				if trimmed == "" || strings.Contains(trimmed, ".") {
					return fmt.Errorf("invalid key %q; expected repos.<name>.default-base", key)
				}
				if cfg.Repos == nil {
					return nil
				}
				repoCfg, ok := cfg.Repos[trimmed]
				if !ok {
					return nil
				}
				fmt.Println(repoCfg.DefaultBase)

			default:
				return fmt.Errorf("unknown key %q; valid keys: projects-dir, repo-search-dirs, editor, repos.<name>.default-base", key)
			}

			return nil
		},
	}
}
