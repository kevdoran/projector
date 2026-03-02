package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
)

func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Unset (clear) an optional config value",
		Long: `Remove or clear an optional configuration value.

Unsettable keys:
  default-editor                   Clear the default editor setting
  editors.<name>                   Remove an entire custom editor entry
  repos.<name>.default-base        Remove a per-repo default base override

Required keys (cannot be unset):
  projects-dir
  repo-search-dirs`,
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
			case key == "projects-dir", key == "repo-search-dirs":
				return fmt.Errorf("cannot unset required key %q", key)

			case key == "default-editor":
				cfg.DefaultEditor = ""

			case strings.HasPrefix(key, "editors."):
				editorName := strings.TrimPrefix(key, "editors.")
				// Reject sub-field keys like editors.<name>.command — unset removes the whole entry
				if editorName == "" || strings.Contains(editorName, ".") {
					return fmt.Errorf("invalid key %q; use editors.<name> to remove an entire editor entry", key)
				}
				if cfg.Editors == nil {
					return fmt.Errorf("editor %q not found", editorName)
				}
				if _, ok := cfg.Editors[editorName]; !ok {
					return fmt.Errorf("editor %q not found", editorName)
				}
				delete(cfg.Editors, editorName)

			case strings.HasPrefix(key, "repos.") && strings.HasSuffix(key, ".default-base"):
				repoName := strings.TrimPrefix(key, "repos.")
				repoName = strings.TrimSuffix(repoName, ".default-base")
				if repoName == "" || strings.Contains(repoName, ".") {
					return fmt.Errorf("invalid key %q; expected repos.<name>.default-base", key)
				}
				if cfg.Repos == nil {
					return fmt.Errorf("repo %q not found", repoName)
				}
				if _, ok := cfg.Repos[repoName]; !ok {
					return fmt.Errorf("repo %q not found", repoName)
				}
				delete(cfg.Repos, repoName)

			default:
				return fmt.Errorf("unknown or non-unsettable key %q; unsettable keys: default-editor, editors.<name>, repos.<name>.default-base", key)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("Unset %s\n", key)
			return nil
		},
	}
}
