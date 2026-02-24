package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/tui"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pj",
		Short: "pj is for managing parallel projects backed by git worktrees",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip version check for help flags
			if cmd.Name() == "help" {
				return nil
			}
			return git.MinVersionCheck()
		},
	}

	root.AddCommand(newProjectsCmd())

	// Rename Cobra's default "completion" command to "autocomp".
	// InitDefaultCompletionCmd adds it now so we can rename it before Execute()
	// re-runs the same logic. DisableDefaultCmd=true prevents Execute() from
	// adding a second "completion" command on top of the renamed one.
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "completion" {
			cmd.Use = "autocomp"
			break
		}
	}
	root.CompletionOptions.DisableDefaultCmd = true

	return root
}

// loadOrInitConfig loads the global config, running first-time setup if needed.
func loadOrInitConfig() (*config.GlobalConfig, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	if err != config.ErrNotFound {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// First-time setup
	fmt.Println("No projector configuration found. Let's set it up!")
	cfg, err = tui.InitConfig()
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	fmt.Println("Configuration saved.")
	return cfg, nil
}
