package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
)

func newDeleteCmd() *cobra.Command {
	var (
		deleteBranches bool
		force          bool
		yes            bool
	)

	cmd := &cobra.Command{
		Use:   "delete [project]",
		Short: "Permanently delete a project and its worktrees",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			projectDir, p, err := resolveProject(cfg.ProjectsDir, args)
			if err != nil {
				return err
			}

			// Collect worktrees — live for active projects, archived records otherwise.
			type worktreeEntry struct {
				repoName     string
				repoPath     string
				worktreePath string
				branch       string
			}
			var worktrees []worktreeEntry

			if p.Project.Status == project.StatusActive {
				wts, err := project.DiscoverWorktrees(projectDir)
				if err != nil {
					return fmt.Errorf("discover worktrees: %w", err)
				}
				for _, wt := range wts {
					worktrees = append(worktrees, worktreeEntry{
						repoName:     wt.RepoName,
						repoPath:     wt.RepoPath,
						worktreePath: wt.WorktreePath,
						branch:       wt.Branch,
					})
				}
			} else {
				for _, wt := range p.ArchivedWorktrees {
					worktrees = append(worktrees, worktreeEntry{
						repoName:     wt.RepoName,
						repoPath:     wt.RepoPath,
						worktreePath: wt.WorktreePath,
						branch:       wt.Branch,
					})
				}
			}

			// Safety checks.
			if p.Project.Status == project.StatusActive {
				for _, wt := range worktrees {
					clean, lines, err := git.StatusPorcelain(wt.worktreePath)
					if err != nil {
						return fmt.Errorf("status check for %s: %w", wt.repoName, err)
					}
					if !clean {
						fmt.Fprintf(os.Stderr, "error: %s has uncommitted changes or untracked files:\n", wt.repoName)
						for _, l := range lines {
							fmt.Fprintf(os.Stderr, "  %s\n", l)
						}
						printManualDeleteHint(projectDir)
						return fmt.Errorf("refusing to delete project with unsaved work; commit or stash changes first")
					}
				}
			}

			var worktreePaths []string
			for _, wt := range worktrees {
				worktreePaths = append(worktreePaths, wt.worktreePath)
			}
			unexpected, err := unexpectedProjectDirEntries(projectDir, worktreePaths)
			if err != nil {
				return fmt.Errorf("scan project dir: %w", err)
			}
			if len(unexpected) > 0 {
				fmt.Fprintf(os.Stderr, "error: project directory contains unexpected files:\n")
				for _, name := range unexpected {
					fmt.Fprintf(os.Stderr, "  %s\n", filepath.Join(projectDir, name))
				}
				printManualDeleteHint(projectDir)
				return fmt.Errorf("refusing to delete project directory with unexpected contents")
			}

			if deleteBranches && !force {
				for _, wt := range worktrees {
					unpushed, err := git.HasUnpushedCommits(wt.repoPath, wt.branch)
					if err != nil {
						return fmt.Errorf("check unpushed commits for %s: %w", wt.repoName, err)
					}
					if unpushed {
						printManualDeleteHint(projectDir)
						return fmt.Errorf(
							"%s branch %q has unpushed commits; push or use --force to delete anyway",
							wt.repoName, wt.branch,
						)
					}
				}
			}

			// Confirmation prompt with preview of actions.
			if !yes {
				fmt.Fprintln(os.Stderr, "The following actions will be performed:")
				fmt.Fprintln(os.Stderr)
				if len(worktrees) == 0 {
					fmt.Fprintln(os.Stderr, "  (no git worktrees to clean up)")
				} else {
					for _, wt := range worktrees {
						fmt.Fprintf(os.Stderr, "  %s:\n", wt.repoName)
						hasAction := false
						if p.Project.Status == project.StatusActive {
							fmt.Fprintf(os.Stderr, "    git worktree remove %s\n", wt.worktreePath)
							hasAction = true
						}
						if deleteBranches {
							fmt.Fprintf(os.Stderr, "    git branch -D %s  (in %s)\n", wt.branch, wt.repoPath)
							hasAction = true
						}
						if !hasAction {
							fmt.Fprintln(os.Stderr, "    (nothing to do)")
						}
					}
				}
				fmt.Fprintln(os.Stderr)
				fmt.Fprintf(os.Stderr, "  rm -rf %s\n", projectDir)
				fmt.Fprintln(os.Stderr)

				fmt.Fprintln(os.Stderr, "Alternatives:")
				fmt.Fprintf(os.Stderr, "  - To archive (reversible): pj project archive %s\n", p.Project.Name)
				if deleteBranches {
					fmt.Fprintln(os.Stderr, "  - To clean up manually, remove the worktrees and branches listed above")
				} else {
					fmt.Fprintln(os.Stderr, "  - To clean up manually, remove the worktrees and directory listed above")
				}
				fmt.Fprintln(os.Stderr)

				fmt.Fprint(os.Stderr, "Proceed? [y/N]: ")
				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			// Execute.
			if p.Project.Status == project.StatusActive {
				for _, wt := range worktrees {
					fmt.Printf("  Removing worktree for %s (this may take a while)...\n", wt.repoName)
					if err := git.WorktreeRemove(wt.repoPath, wt.worktreePath); err != nil {
						return fmt.Errorf("remove worktree for %s: %w", wt.repoName, err)
					}
				}
			}

			if deleteBranches {
				for _, wt := range worktrees {
					fmt.Printf("  Deleting branch %q in %s...\n", wt.branch, wt.repoName)
					if _, err := git.RunGit(wt.repoPath, "branch", "-D", wt.branch); err != nil {
						return fmt.Errorf("delete branch %q in %s: %w", wt.branch, wt.repoName, err)
					}
				}
			}

			fmt.Printf("  Removing project directory...\n")
			if err := os.RemoveAll(projectDir); err != nil {
				return fmt.Errorf("remove project dir: %w", err)
			}

			fmt.Printf("Project %q deleted.\n", p.Project.Name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&deleteBranches, "delete-branches", false, "Also delete the git branch for each repo")
	cmd.Flags().BoolVar(&force, "force", false, "Skip unpushed commits check when deleting branches")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")

	return cmd
}

// printManualDeleteHint reminds the user that they can always delete the project
// directory manually if the safety checks are preventing an automated delete.
func printManualDeleteHint(projectDir string) {
	fmt.Fprintf(os.Stderr, "hint: you can always delete the project manually: rm -rf %s\n", projectDir)
}

// unexpectedProjectDirEntries returns names of entries in projectDir that are
// neither .projector.toml nor one of the given worktree paths.
func unexpectedProjectDirEntries(projectDir string, worktreePaths []string) ([]string, error) {
	expected := map[string]bool{
		".projector.toml": true,
	}
	for _, p := range worktreePaths {
		expected[filepath.Base(p)] = true
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var unexpected []string
	for _, entry := range entries {
		if !expected[entry.Name()] {
			unexpected = append(unexpected, entry.Name())
		}
	}
	return unexpected, nil
}
