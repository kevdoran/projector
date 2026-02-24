package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
	"github.com/kevdoran/projector/internal/repo"
	"github.com/kevdoran/projector/internal/tui"
)

func newAddRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-repo [repos...]",
		Short: "Add one or more repositories to a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			// Resolve project name and repo args from positional arguments.
			// Ambiguity rules:
			//   2+ args → args[0] = project, rest = repos
			//   1 arg   → if it matches a known project, use as project + interactive repo select
			//             else treat as repo, detect project from CWD
			//   0 args  → detect project from CWD, interactive repo select
			var projectName string
			var repoArgs []string

			switch {
			case len(args) >= 2:
				projectName = args[0]
				repoArgs = args[1:]

			case len(args) == 1:
				// Check if arg matches a known project
				candidate := args[0]
				candidateDir := project.ProjectDir(cfg.ProjectsDir, candidate)
				if _, err := project.Load(candidateDir); err == nil {
					projectName = candidate
					// interactive repo selection below
				} else {
					// treat as repo, detect project from CWD
					repoArgs = []string{candidate}
				}

			case len(args) == 0:
				// detect project from CWD
			}

			if projectName == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get cwd: %w", err)
				}
				projectDir, err := project.FindProjectDir(cwd)
				if err != nil {
					return fmt.Errorf("could not detect project from current directory: %w", err)
				}
				p, err := project.Load(projectDir)
				if err != nil {
					return fmt.Errorf("load project: %w", err)
				}
				projectName = p.Project.Name
			}

			projectDir := project.ProjectDir(cfg.ProjectsDir, projectName)
			p, err := project.Load(projectDir)
			if err != nil {
				return fmt.Errorf("load project %q: %w", projectName, err)
			}
			if p.Project.Status != project.StatusActive {
				return fmt.Errorf("project %q is not active", projectName)
			}

			// Discover existing worktrees to avoid duplicates
			existing, err := project.DiscoverWorktrees(projectDir)
			if err != nil {
				return fmt.Errorf("discover existing worktrees: %w", err)
			}
			existingNames := map[string]bool{}
			for _, wt := range existing {
				existingNames[wt.RepoName] = true
			}
			existingNamesList := make([]string, 0, len(existingNames))
			for name := range existingNames {
				existingNamesList = append(existingNamesList, name)
			}

			// Resolve repos to add
			var repos []repo.Repo
			if len(repoArgs) > 0 {
				repos, err = repo.ResolveRepos(repoArgs, cfg.RepoSearchDirs)
				if err != nil {
					return fmt.Errorf("resolve repos: %w", err)
				}
			} else {
				// Interactive selection, excluding already-added repos
				discovered, err := repo.Discover(cfg.RepoSearchDirs)
				if err != nil {
					return fmt.Errorf("discover repos: %w", err)
				}
				repos, err = tui.SelectRepos(discovered, existingNamesList)
				if err != nil {
					if errors.Is(err, tui.ErrAborted) {
						fmt.Println("Aborted.")
						return nil
					}
					return fmt.Errorf("select repos: %w", err)
				}
			}

			now := time.Now().UTC()

			for _, r := range repos {
				if existingNames[r.Name] {
					fmt.Fprintf(os.Stderr, "warning: %s is already in project, skipping\n", r.Name)
					continue
				}

				base, err := config.ResolveBase(cfg, r.Name, r.Path)
				if err != nil {
					return fmt.Errorf("resolve base for %s: %w", r.Name, err)
				}

				branchName, err := git.AvailableBranchName(r.Path, projectName, now)
				if err != nil {
					return fmt.Errorf("branch name for %s: %w", r.Name, err)
				}

				worktreePath := filepath.Join(projectDir, r.Name+"+"+projectName)
				if err := git.WorktreeAdd(r.Path, worktreePath, base, branchName, true); err != nil {
					return fmt.Errorf("add worktree for %s: %w", r.Name, err)
				}
				fmt.Printf("  added worktree: %s (branch: %s)\n", worktreePath, branchName)
			}

			return nil
		},
	}

	return cmd
}
