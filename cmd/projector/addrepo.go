package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/config"
	"github.com/kevdoran/projector/internal/git"
	"github.com/kevdoran/projector/internal/project"
	"github.com/kevdoran/projector/internal/repo"
	"github.com/kevdoran/projector/internal/tui"
)

func newAddRepoCmd() *cobra.Command {
	var (
		detached bool
		base     string
		checkout bool
	)

	cmd := &cobra.Command{
		Use:   "add-repo [repos...]",
		Short: "Add one or more repositories to a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
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
				if len(discovered) == 0 {
					printNoReposFound(cfg.RepoSearchDirs)
					return nil
				}
				repos, err = tui.SelectRepos(discovered, existingNamesList, "")
				if err != nil {
					if errors.Is(err, tui.ErrAborted) {
						fmt.Println("Aborted.")
						return nil
					}
					return fmt.Errorf("select repos: %w", err)
				}
			}

			now := time.Now().UTC()

			// Phase 1: Resolve base refs, fetch remotes, and check ref existence.
			type resolvedRepo struct {
				repo             repo.Repo
				base             string
				refFound         bool
				branchInUse      bool
				conflictWorktree string
			}
			var resolved []resolvedRepo

			for _, r := range repos {
				if existingNames[r.Name] {
					fmt.Fprintf(os.Stderr, "warning: %s is already in project, skipping\n", r.Name)
					continue
				}

				var repoBase string
				if base != "" {
					repoBase = base
				} else {
					repoBase, err = config.ResolveBase(cfg, r.Name, r.Path)
					if err != nil {
						return fmt.Errorf("resolve base for %s: %w", r.Name, err)
					}
				}

				// Fetch before using a remote-tracking ref so it is up to date.
				remote, err := git.RemoteForRef(r.Path, repoBase)
				if err != nil {
					return fmt.Errorf("check remote for %s: %w", r.Name, err)
				}
				if remote != "" {
					fmt.Printf("  fetching %s in %s…\n", remote, r.Name)
					if err := git.Fetch(r.Path, remote); err != nil {
						return fmt.Errorf("fetch %s in %s: %w", remote, r.Name, err)
					}
				}

				refFound := true
				if base != "" {
					exists, err := git.RefExists(r.Path, repoBase)
					if err != nil {
						return fmt.Errorf("check ref for %s: %w", r.Name, err)
					}
					refFound = exists
				}

				// Check if the branch is already checked out in another worktree.
				branchInUse := false
				conflictWorktree := ""
				if checkout && refFound {
					branchName, err := git.BranchNameFromRef(r.Path, repoBase)
					if err != nil {
						return fmt.Errorf("resolve branch name for %s: %w", r.Name, err)
					}
					wtPath, err := git.WorktreeForBranch(r.Path, branchName)
					if err != nil {
						return fmt.Errorf("check worktrees for %s: %w", r.Name, err)
					}
					branchInUse = wtPath != ""
					conflictWorktree = wtPath
				}

				resolved = append(resolved, resolvedRepo{repo: r, base: repoBase, refFound: refFound, branchInUse: branchInUse, conflictWorktree: conflictWorktree})
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
					return fmt.Errorf("base %q was not found in any of the selected repositories", base)
				}

				if len(missing) > 0 {
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
						fmt.Fprintln(os.Stderr, "Aborted.")
						return nil
					}

					for i := range resolved {
						if !resolved[i].refFound {
							fallback, _ := config.ResolveBase(cfg, resolved[i].repo.Name, resolved[i].repo.Path)
							resolved[i].base = fallback
						}
					}
				}
			}

			// Check for branches already checked out in existing worktrees.
			if checkout {
				var conflicts []resolvedRepo
				for _, rr := range resolved {
					if rr.branchInUse {
						conflicts = append(conflicts, rr)
					}
				}
				if len(conflicts) > 0 {
					var buf strings.Builder
					buf.WriteString("cannot checkout: branch is already checked out in an existing worktree:\n\n")
					tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
					fmt.Fprintln(tw, "  PROJECT\tREPO\tBRANCH\tREMOTE REF\tWORKTREE")
					for _, rr := range conflicts {
						branchName, _ := git.BranchNameFromRef(rr.repo.Path, rr.base)
						remoteRef := "-"
						if branchName != rr.base {
							remoteRef = rr.base
						}
						proj := projectNameFromWorktreePath(cfg.ProjectsDir, rr.conflictWorktree)
						fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", proj, rr.repo.Name, branchName, remoteRef, rr.conflictWorktree)
					}
					tw.Flush()
					buf.WriteString("\nRetry without --checkout to create new branches, or use --detached to use detached HEAD state.")
					return fmt.Errorf("%s", buf.String())
				}
			}

			// Phase 3: Create worktrees.
			for _, rr := range resolved {
				worktreePath := filepath.Join(projectDir, rr.repo.Name+"+"+projectName)

				if detached {
					if err := git.WorktreeAddDetached(rr.repo.Path, worktreePath, rr.base); err != nil {
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					fmt.Printf("  added worktree: %s (detached at %s)\n", worktreePath, rr.base)
				} else if checkout {
					branchName, err := git.BranchNameFromRef(rr.repo.Path, rr.base)
					if err != nil {
						return fmt.Errorf("resolve branch name for %s: %w", rr.repo.Name, err)
					}
					if err := git.WorktreeAdd(rr.repo.Path, worktreePath, "", branchName, false); err != nil {
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					fmt.Printf("  added worktree: %s (checkout: %s)\n", worktreePath, branchName)
				} else {
					branchName, err := git.AvailableBranchName(rr.repo.Path, projectName, now)
					if err != nil {
						return fmt.Errorf("branch name for %s: %w", rr.repo.Name, err)
					}

					if err := git.WorktreeAdd(rr.repo.Path, worktreePath, rr.base, branchName, true); err != nil {
						return fmt.Errorf("add worktree for %s: %w", rr.repo.Name, err)
					}
					fmt.Printf("  added worktree: %s (branch: %s)\n", worktreePath, branchName)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&detached, "detached", false, "Create worktrees in detached HEAD state (no branch)")
	cmd.Flags().StringVar(&base, "base", "", "Git ref to branch from (branch, tag, SHA, or remote ref such as origin/main)")
	cmd.Flags().BoolVar(&checkout, "checkout", false, "Check out an existing branch instead of creating a new one (requires --base)")

	return cmd
}
