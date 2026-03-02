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
  projects-dir                     Path where project directories are created
  repo-search-dirs                 Comma-separated list of repo search directories
  default-editor                   Default editor command for 'pj project open'
  editors.<name>.name              Display name for a custom editor
  editors.<name>.command           Executable command for a custom editor
  editors.<name>.terminal          Whether editor is terminal-mode ("true"/"false")
  repos.<name>.default-base        Per-repo default branch base`,
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

			case key == "default-editor":
				fmt.Println(cfg.DefaultEditor)

			case strings.HasPrefix(key, "editors."):
				trimmed := strings.TrimPrefix(key, "editors.")
				parts := strings.SplitN(trimmed, ".", 2)
				if len(parts) != 2 || parts[0] == "" {
					return fmt.Errorf("invalid key %q; expected editors.<name>.<field>", key)
				}
				editorName := parts[0]
				field := parts[1]
				if cfg.Editors == nil {
					return nil
				}
				ec, ok := cfg.Editors[editorName]
				if !ok {
					return nil
				}
				switch field {
				case "name":
					fmt.Println(ec.Name)
				case "command":
					fmt.Println(ec.Command)
				case "terminal":
					fmt.Println(ec.Terminal)
				default:
					return fmt.Errorf("unknown editor field %q; valid fields: name, command, terminal", field)
				}

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
				return fmt.Errorf("unknown key %q; valid keys: projects-dir, repo-search-dirs, default-editor, editors.<name>.<field>, repos.<name>.default-base", key)
			}

			return nil
		},
	}
}
