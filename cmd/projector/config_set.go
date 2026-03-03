package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/tui"
)

func newConfigSetCmd() *cobra.Command {
	var (
		addFlag    bool
		removeFlag bool
	)

	cmd := &cobra.Command{
		Use:   "set [flags] <key> <value>",
		Short: "Set a config value",
		Long: `Set an individual configuration key-value pair.

Supported keys:
  projects-dir                     Path where project directories are created
  repo-search-dirs                 Comma-separated list of repo search directories
  default-editor                   Default editor command for 'pj project open'
  editors.<name>.name              Display name for a custom editor
  editors.<name>.command           Executable command for a custom editor
  editors.<name>.terminal          "true"/"false" — print cd+command instead of launching
  repos.<name>.default-base        Per-repo default branch base

Flags --add and --remove can be used with repo-search-dirs to append or remove
a single entry instead of replacing the entire list.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			key := args[0]
			value := args[1]

			cfg, err := config.Load()
			if err != nil {
				if err == config.ErrNotFound {
					return fmt.Errorf("no configuration found; run 'pj config setup' to set up pj")
				}
				return fmt.Errorf("load config: %w", err)
			}

			if addFlag && removeFlag {
				return fmt.Errorf("--add and --remove cannot be used together")
			}

			if (addFlag || removeFlag) && key != "repo-search-dirs" {
				return fmt.Errorf("--add and --remove can only be used with repo-search-dirs")
			}

			switch {
			case key == "projects-dir":
				cfg.ProjectsDir = tui.ExpandHome(value)
				fmt.Printf("Set projects-dir = %s\n", cfg.ProjectsDir)

			case key == "repo-search-dirs":
				if addFlag {
					expanded := tui.ExpandHome(value)
					cfg.RepoSearchDirs = append(cfg.RepoSearchDirs, expanded)
					fmt.Printf("Added %s to repo-search-dirs\n", expanded)
				} else if removeFlag {
					expanded := tui.ExpandHome(value)
					found := false
					var updated []string
					for _, dir := range cfg.RepoSearchDirs {
						if dir == expanded {
							found = true
							continue
						}
						updated = append(updated, dir)
					}
					if !found {
						return fmt.Errorf("%s is not in repo-search-dirs", expanded)
					}
					cfg.RepoSearchDirs = updated
					fmt.Printf("Removed %s from repo-search-dirs\n", expanded)
				} else {
					var dirs []string
					for _, d := range strings.Split(value, ",") {
						d = strings.TrimSpace(d)
						if d != "" {
							dirs = append(dirs, tui.ExpandHome(d))
						}
					}
					cfg.RepoSearchDirs = dirs
					fmt.Printf("Set repo-search-dirs = %s\n", strings.Join(dirs, ", "))
				}

			case key == "default-editor":
				cfg.DefaultEditor = value
				fmt.Printf("Set default-editor = %s\n", cfg.DefaultEditor)

			case strings.HasPrefix(key, "editors."):
				// editors.<name>.name, editors.<name>.command, editors.<name>.terminal
				trimmed := strings.TrimPrefix(key, "editors.")
				parts := strings.SplitN(trimmed, ".", 2)
				if len(parts) != 2 || parts[0] == "" {
					return fmt.Errorf("invalid key %q; expected editors.<name>.<field>", key)
				}
				editorName := parts[0]
				field := parts[1]
				if cfg.Editors == nil {
					cfg.Editors = make(map[string]config.EditorConfig)
				}
				ec := cfg.Editors[editorName]
				switch field {
				case "name":
					ec.Name = value
				case "command":
					ec.Command = value
				case "terminal":
					ec.Terminal = value == "true"
				default:
					return fmt.Errorf("unknown editor field %q; valid fields: name, command, terminal", field)
				}
				cfg.Editors[editorName] = ec
				fmt.Printf("Set %s = %s\n", key, value)

			case strings.HasPrefix(key, "repos.") && strings.HasSuffix(key, ".default-base"):
				// Extract repo name from repos.<name>.default-base
				trimmed := strings.TrimPrefix(key, "repos.")
				trimmed = strings.TrimSuffix(trimmed, ".default-base")
				if trimmed == "" || strings.Contains(trimmed, ".") {
					return fmt.Errorf("invalid key %q; expected repos.<name>.default-base", key)
				}
				if cfg.Repos == nil {
					cfg.Repos = make(map[string]config.RepoConfig)
				}
				cfg.Repos[trimmed] = config.RepoConfig{DefaultBase: value}
				fmt.Printf("Set repos.%s.default-base = %s\n", trimmed, value)

			default:
				return fmt.Errorf("unknown key %q; valid keys: projects-dir, repo-search-dirs, default-editor, editors.<name>.<field>, repos.<name>.default-base", key)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&addFlag, "add", false, "Append a value to an array key (repo-search-dirs)")
	cmd.Flags().BoolVar(&removeFlag, "remove", false, "Remove a value from an array key (repo-search-dirs)")

	return cmd
}
