package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
)

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Display current configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cfg, err := config.Load()
			if err != nil {
				if err == config.ErrNotFound {
					return fmt.Errorf("no configuration found; run 'pj config setup' to set up pj")
				}
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.ProjectsDir != "" {
				fmt.Printf("projects-dir=%s\n", cfg.ProjectsDir)
			}

			for i, dir := range cfg.RepoSearchDirs {
				fmt.Printf("repo-search-dirs.%d=%s\n", i, dir)
			}

			if cfg.DefaultEditor != "" {
				fmt.Printf("default-editor=%s\n", cfg.DefaultEditor)
			}

			if len(cfg.Editors) > 0 {
				editorNames := make([]string, 0, len(cfg.Editors))
				for name := range cfg.Editors {
					editorNames = append(editorNames, name)
				}
				sort.Strings(editorNames)

				for _, name := range editorNames {
					ec := cfg.Editors[name]
					if ec.Name != "" {
						fmt.Printf("editors.%s.name=%s\n", name, ec.Name)
					}
					if ec.Command != "" {
						fmt.Printf("editors.%s.command=%s\n", name, ec.Command)
					}
					if ec.Terminal {
						fmt.Printf("editors.%s.terminal=true\n", name)
					}
				}
			}

			if len(cfg.Repos) > 0 {
				// Sort repo names for stable output.
				names := make([]string, 0, len(cfg.Repos))
				for name := range cfg.Repos {
					names = append(names, name)
				}
				sort.Strings(names)

				for _, name := range names {
					repoCfg := cfg.Repos[name]
					if repoCfg.DefaultBase != "" {
						fmt.Printf("repos.%s.default-base=%s\n", name, repoCfg.DefaultBase)
					}
				}
			}

			return nil
		},
	}
}
