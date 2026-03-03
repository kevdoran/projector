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

// builtinEditors is the ordered list of editors offered in the selection prompt.
// Each entry must accept a directory path as its first positional argument.
// "finder" is a special case handled by `open /path` on macOS.
var builtinEditors = []tui.EditorOption{
	{Name: "Cursor", Command: "cursor"},
	{Name: "VS Code", Command: "code"},
	{Name: "Windsurf", Command: "windsurf"},
	{Name: "Zed", Command: "zed"},
	{Name: "Sublime Text", Command: "subl"},
	{Name: "BBEdit", Command: "bbedit"},
	{Name: "IntelliJ IDEA", Command: "idea"},
	{Name: "Claude Code", Command: "claude", Terminal: true},
	{Name: "OpenCode", Command: "opencode", Terminal: true},
	{Name: "Finder (macOS)", Command: "finder"},
}

func newOpenCmd() *cobra.Command {
	var editorFlag string

	cmd := &cobra.Command{
		Use:   "open [project] [-e editor]",
		Short: "Open a project in an editor or IDE",
		Long: `Open a project in an editor or IDE.

If no project is specified, the current directory is used to resolve the project.
Prompts for an editor unless --editor/-e is given or default-editor is set in config.

To always open with a specific editor without being prompted:
  pj config set default-editor <command>    (e.g. cursor, code, claude)
  pj config unset default-editor            (restore the prompt)`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			projectDir, _, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			// Build the full editor list (builtins + custom config entries).
			allEditors := buildEditorList(cfg)

			// Determine which editor to use.
			var editorCmd string
			switch {
			case editorFlag != "":
				editorCmd = editorFlag
			case cfg.DefaultEditor != "":
				editorCmd = cfg.DefaultEditor
			default:
				// Interactive selection — only show installed editors.
				installed := filterInstalled(allEditors)
				if len(installed) == 0 {
					return fmt.Errorf("no supported editors found on PATH; use --editor to specify one")
				}
				choice, err := tui.SelectEditor(installed)
				if err != nil {
					if errors.Is(err, tui.ErrAborted) {
						fmt.Println("Aborted.")
						return nil
					}
					return fmt.Errorf("select editor: %w", err)
				}
				editorCmd = choice
			}

			// Look up editor metadata for terminal mode.
			editorMeta := findEditor(allEditors, editorCmd)
			if err := launchEditor(editorMeta, projectDir); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&editorFlag, "editor", "e", "", "editor command to use (skips prompt)")

	return cmd
}

// buildEditorList merges builtinEditors with custom editors from config.
// Config entries override builtins by command key.
func buildEditorList(cfg *config.GlobalConfig) []tui.EditorOption {
	// Start with builtins; config entries can override by command.
	byCommand := make(map[string]int, len(builtinEditors))
	result := make([]tui.EditorOption, len(builtinEditors))
	copy(result, builtinEditors)
	for i, e := range result {
		byCommand[e.Command] = i
	}

	for key, ec := range cfg.Editors {
		command := ec.Command
		if command == "" {
			command = key
		}
		name := ec.Name
		if name == "" {
			name = key
		}
		opt := tui.EditorOption{
			Name:     name,
			Command:  command,
			Terminal: ec.Terminal,
		}
		if idx, ok := byCommand[command]; ok {
			// Override the builtin.
			result[idx] = opt
		} else {
			byCommand[command] = len(result)
			result = append(result, opt)
		}
	}

	return result
}

// filterInstalled returns only the editors whose commands are found on PATH.
func filterInstalled(editors []tui.EditorOption) []tui.EditorOption {
	var installed []tui.EditorOption
	for _, e := range editors {
		if e.Command == "finder" {
			// Finder is always available on macOS.
			installed = append(installed, e)
		} else if _, err := exec.LookPath(e.Command); err == nil {
			installed = append(installed, e)
		}
	}
	return installed
}

// findEditor looks up an editor by command in the list. If not found, returns
// a default EditorOption with the given command.
func findEditor(editors []tui.EditorOption, command string) tui.EditorOption {
	for _, e := range editors {
		if e.Command == command {
			return e
		}
	}
	return tui.EditorOption{Name: command, Command: command}
}

// launchEditor opens projectDir in the specified editor.
func launchEditor(editor tui.EditorOption, projectDir string) error {
	if editor.Terminal {
		fmt.Printf("To open in %s, run:\n\n  cd %q && %s\n", editor.Name, projectDir, editor.Command)
		return nil
	}

	var cmd *exec.Cmd
	if editor.Command == "finder" {
		cmd = exec.Command("open", projectDir)
	} else {
		cmd = exec.Command(editor.Command, projectDir)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", editor.Command, err)
	}
	// Don't wait — GUI apps return immediately from Start and run independently.
	return nil
}
