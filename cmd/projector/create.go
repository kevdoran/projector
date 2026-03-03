package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		detached    bool
		checkout    bool
	)

	cmd := &cobra.Command{
		Use:   "create <name> [repos...]",
		Short: "Create a new project",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			repoArgs := args[1:]

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Validate flag combinations
			if checkout && detached {
				return fmt.Errorf("--checkout and --detached are mutually exclusive")
			}
			if checkout && base == "" {
				return fmt.Errorf("--checkout requires --base to specify which branch to check out")
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
				if len(discovered) == 0 {
					printNoReposFound(cfg.RepoSearchDirs)
					return nil
				}
				repos, err = tui.SelectRepos(discovered, nil, "use --empty to create an empty project")
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

			// Phase 1: Resolve base refs for each repo and fetch remotes.
			type resolvedRepo struct {
				repo     repo.Repo
				base     string
				refFound bool // whether the explicit --base ref was found
			}
			resolved := make([]resolvedRepo, 0, len(repos))

			for _, r := range repos {
				var repoBase string
				switch {
				case base != "":
					repoBase = base

				case fromProject != "":
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

				// Check if the explicit --base ref exists in this repo.
				refFound := true
				if base != "" {
					exists, err := git.RefExists(r.Path, repoBase)
					if err != nil {
						rollback()
						return fmt.Errorf("check ref for %s: %w", r.Name, err)
					}
					refFound = exists
				}

				resolved = append(resolved, resolvedRepo{repo: r, base: repoBase, refFound: refFound})
			}

			// Phase 2: Pre-validate base ref across repos when --base is set.
			if base != "" && len(resolved) > 0 {
				var found, missing []resolvedRepo
				for _, rr := range resolved {
					if rr.refFound {
						found = append(found, rr)
					} else {
						missing = append(missing, rr)
					}
				}

				if len(found) == 0 {
					// None have the ref — abort.
					rollback()
					return fmt.Errorf("base %q was not found in any of the selected repositories", base)
				}

				if len(missing) > 0 {
					// Some repos missing the ref — prompt for confirmation.
					fmt.Fprintf(os.Stderr, "Warning: base %q not found in all repositories:\n", base)
					for _, rr := range resolved {
						if rr.refFound {
							fmt.Fprintf(os.Stderr, "  %s: found\n", rr.repo.Name)
						} else {
							fallback, _ := config.ResolveBase(cfg, rr.repo.Name, rr.repo.Path)
							fmt.Fprintf(os.Stderr, "  %s: not found (will use %s)\n", rr.repo.Name, fallback)
						}
					}
					fmt.Fprint(os.Stderr, "\nProceed? [y/N]: ")
					reader := bufio.NewReader(os.Stdin)
					response, _ := reader.ReadString('\n')
					if strings.ToLower(strings.TrimSpace(response)) != "y" {
						rollback()
						fmt.Fprintln(os.Stderr, "Aborted. You can add repos individually with: pj project add-repo")
						return nil
					}

					// Apply fallback bases for repos that are missing the ref.
					for i := range resolved {
						if !resolved[i].refFound {
							fallback, _ := config.ResolveBase(cfg, resolved[i].repo.Name, resolved[i].repo.Path)
							resolved[i].base = fallback
						}
					}
				}
			}

			// Phase 3: Create worktrees.
			for _, rr := range resolved {
				worktreePath := filepath.Join(projectDir, rr.repo.Name+"+"+name)

				if detached {
					if err := git.WorktreeAddDetached(rr.repo.Path, worktreePath, rr.base); err != nil {
						rollback()
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					created = append(created, struct {
						repoPath     string
						worktreePath string
					}{rr.repo.Path, worktreePath})
					fmt.Printf("  created worktree: %s (detached at %s)\n", worktreePath, rr.base)
				} else if checkout {
					branchName, err := git.BranchNameFromRef(rr.repo.Path, rr.base)
					if err != nil {
						rollback()
						return fmt.Errorf("resolve branch name for %s: %w", rr.repo.Name, err)
					}
					if err := git.WorktreeAdd(rr.repo.Path, worktreePath, "", branchName, false); err != nil {
						rollback()
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					created = append(created, struct {
						repoPath     string
						worktreePath string
					}{rr.repo.Path, worktreePath})
					fmt.Printf("  created worktree: %s (checkout: %s)\n", worktreePath, branchName)
				} else {
					branchName, err := git.AvailableBranchName(rr.repo.Path, name, now)
					if err != nil {
						rollback()
						return fmt.Errorf("branch name for %s: %w", rr.repo.Name, err)
					}

					if err := git.WorktreeAdd(rr.repo.Path, worktreePath, rr.base, branchName, true); err != nil {
						rollback()
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					created = append(created, struct {
						repoPath     string
						worktreePath string
					}{rr.repo.Path, worktreePath})
					fmt.Printf("  created worktree: %s (branch: %s)\n", worktreePath, branchName)
				}
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
	cmd.Flags().BoolVar(&detached, "detached", false, "Create worktrees in detached HEAD state (no branch)")
	cmd.Flags().BoolVar(&checkout, "checkout", false, "Check out an existing branch instead of creating a new one (requires --base)")

	return cmd
}
