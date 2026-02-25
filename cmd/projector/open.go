package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/tui"
)

// supportedEditors is the ordered list of editors offered in the selection prompt.
// Each entry must accept a directory path as its first positional argument.
// "finder" is a special case handled by `open /path` on macOS.
var supportedEditors = []tui.EditorOption{
	{Name: "Cursor", Command: "cursor"},
	{Name: "VS Code", Command: "code"},
	{Name: "Zed", Command: "zed"},
	{Name: "Sublime Text", Command: "subl"},
	{Name: "BBEdit", Command: "bbedit"},
	{Name: "IntelliJ IDEA", Command: "idea"},
	{Name: "Finder (macOS)", Command: "finder"},
}

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [project]",
		Short: "Open a project in the configured editor or IDE",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			projectDir, _, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			// First run: no editor configured — prompt the user.
			if cfg.Editor == "" {
				fmt.Println("No default editor configured for 'pj project open'.")
				options := detectEditors()
				choice, err := tui.SelectEditor(options)
				if err != nil {
					if errors.Is(err, tui.ErrAborted) {
						fmt.Println("Aborted.")
						return nil
					}
					return fmt.Errorf("select editor: %w", err)
				}
				cfg.Editor = choice
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
				fmt.Printf("Default editor set to %q. You can change it in ~/.projector/projector-config.toml\n", cfg.Editor)
			}

			return launchEditor(cfg.Editor, projectDir)
		},
	}
}

// detectEditors annotates supportedEditors with installation status.
func detectEditors() []tui.EditorOption {
	options := make([]tui.EditorOption, len(supportedEditors))
	for i, e := range supportedEditors {
		options[i] = e
		if e.Command == "finder" {
			// Finder is always available on macOS.
			options[i].Installed = true
		} else {
			_, err := exec.LookPath(e.Command)
			options[i].Installed = err == nil
		}
	}
	return options
}

// launchEditor opens projectDir in the configured editor.
func launchEditor(editorCmd, projectDir string) error {
	var cmd *exec.Cmd
	if editorCmd == "finder" {
		cmd = exec.Command("open", projectDir)
	} else {
		cmd = exec.Command(editorCmd, projectDir)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", editorCmd, err)
	}
	// Don't wait — GUI apps return immediately from Start and run independently.
	return nil
}
