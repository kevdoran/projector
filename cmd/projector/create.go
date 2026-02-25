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

func newCreateCmd() *cobra.Command {
	var (
		fromProject string
		empty       bool
		base        string
	)

	cmd := &cobra.Command{
		Use:   "create <name> [repos...]",
		Short: "Create a new project",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			repoArgs := args[1:]

			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			// Validate project name
			if err := project.ValidateName(name); err != nil {
				return err
			}

			// Check for duplicate
			projectDir := project.ProjectDir(cfg.ProjectsDir, name)
			if _, err := os.Stat(projectDir); err == nil {
				return fmt.Errorf("project %q already exists at %s", name, projectDir)
			}

			// Resolve repos
			var repos []repo.Repo
			switch {
			case empty:
				// no repos

			case fromProject != "":
				// Discover repos from the source project's live worktrees
				srcDir := project.ProjectDir(cfg.ProjectsDir, fromProject)
				srcCfg, err := project.Load(srcDir)
				if err != nil {
					return fmt.Errorf("load source project %q: %w", fromProject, err)
				}
				if srcCfg.Project.Status != project.StatusActive {
					return fmt.Errorf("source project %q is not active", fromProject)
				}
				worktrees, err := project.DiscoverWorktrees(srcDir)
				if err != nil {
					return fmt.Errorf("discover worktrees from %q: %w", fromProject, err)
				}
				for _, wt := range worktrees {
					repos = append(repos, repo.Repo{Name: wt.RepoName, Path: wt.RepoPath})
				}

			case len(repoArgs) > 0:
				repos, err = repo.ResolveRepos(repoArgs, cfg.RepoSearchDirs)
				if err != nil {
					return fmt.Errorf("resolve repos: %w", err)
				}

			default:
				// Interactive selection
				discovered, err := repo.Discover(cfg.RepoSearchDirs)
				if err != nil {
					return fmt.Errorf("discover repos: %w", err)
				}
				repos, err = tui.SelectRepos(discovered, nil)
				if err != nil {
					if errors.Is(err, tui.ErrAborted) {
						fmt.Println("Aborted.")
						return nil
					}
					return fmt.Errorf("select repos: %w", err)
				}
			}

			// Create project directory
			if err := os.MkdirAll(projectDir, 0755); err != nil {
				return fmt.Errorf("create project dir: %w", err)
			}

			now := time.Now().UTC()

			// Create worktrees for each repo, tracking rollbacks
			var created []struct {
				repoPath     string
				worktreePath string
			}
			rollback := func() {
				for i := len(created) - 1; i >= 0; i-- {
					c := created[i]
					_ = git.WorktreeRemove(c.repoPath, c.worktreePath)
					_ = os.RemoveAll(c.worktreePath)
				}
				_ = os.RemoveAll(projectDir)
			}

			for _, r := range repos {
				// Determine base ref for this repo.
				var repoBase string
				switch {
				case base != "":
					// Explicit --base: use it for every repo.
					repoBase = base

				case fromProject != "":
					// --from with no --base: inherit the source project's worktree branch.
					srcDir := project.ProjectDir(cfg.ProjectsDir, fromProject)
					worktrees, err := project.DiscoverWorktrees(srcDir)
					if err == nil {
						for _, wt := range worktrees {
							if wt.RepoName == r.Name {
								repoBase = wt.Branch
								break
							}
						}
					}
					if repoBase == "" {
						repoBase, _ = config.ResolveBase(cfg, r.Name, r.Path)
					}

				default:
					repoBase, err = config.ResolveBase(cfg, r.Name, r.Path)
					if err != nil {
						rollback()
						return fmt.Errorf("resolve base for %s: %w", r.Name, err)
					}
				}

				// Fetch before using a remote-tracking ref so it is up to date.
				remote, err := git.RemoteForRef(r.Path, repoBase)
				if err != nil {
					rollback()
					return fmt.Errorf("check remote for %s: %w", r.Name, err)
				}
				if remote != "" {
					fmt.Printf("  fetching %s in %s…\n", remote, r.Name)
					if err := git.Fetch(r.Path, remote); err != nil {
						rollback()
						return fmt.Errorf("fetch %s in %s: %w", remote, r.Name, err)
					}
				}

				// Find available branch name
				branchName, err := git.AvailableBranchName(r.Path, name, now)
				if err != nil {
					rollback()
					return fmt.Errorf("branch name for %s: %w", r.Name, err)
				}

				worktreePath := filepath.Join(projectDir, r.Name+"+"+name)
				if err := git.WorktreeAdd(r.Path, worktreePath, repoBase, branchName, true); err != nil {
					rollback()
					return fmt.Errorf("add worktree for %s: %w", r.Name, err)
				}
				created = append(created, struct {
					repoPath     string
					worktreePath string
				}{r.Path, worktreePath})
				fmt.Printf("  created worktree: %s (branch: %s)\n", worktreePath, branchName)
			}

			// Write .projector.toml
			projCfg := &project.ProjectConfig{
				Project: project.ProjectMeta{
					Name:      name,
					CreatedAt: now,
					Status:    project.StatusActive,
				},
			}
			if err := project.Save(projCfg, projectDir); err != nil {
				rollback()
				return fmt.Errorf("save project config: %w", err)
			}

			fmt.Printf("📽️  Project %q created at %s\n", name, projectDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromProject, "from", "", "Copy repo list from an existing project")
	cmd.Flags().BoolVar(&empty, "empty", false, "Create an empty project with no repos")
	cmd.Flags().StringVar(&base, "base", "", "Git ref to branch from (branch, tag, SHA, or remote ref such as origin/main); remote refs are fetched automatically")

	return cmd
}

