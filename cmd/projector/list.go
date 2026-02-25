package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevdoran/projector/internal/project"
)

func newListCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrInitConfig()
			if err != nil {
				return err
			}

			projects, err := project.ListAll(cfg.ProjectsDir)
			if err != nil {
				return fmt.Errorf("list projects: %w", err)
			}

			if len(projects) == 0 {
				fmt.Println("No projects found. Run 'pj project create <name>' to create one.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			if verbose {
				fmt.Fprintln(w, "PROJECT\tSTATUS\tCREATED\tREPOS\tREPO NAMES")
			} else {
				fmt.Fprintln(w, "PROJECT\tSTATUS\tCREATED\tREPOS")
			}

			for _, p := range projects {
				var repoCount int
				var repoNames []string

				if p.Project.Status == project.StatusActive {
					dir := project.ProjectDir(cfg.ProjectsDir, p.Project.Name)
					worktrees, err := project.DiscoverWorktrees(dir)
					if err == nil {
						repoCount = len(worktrees)
						for _, wt := range worktrees {
							repoNames = append(repoNames, wt.RepoName)
						}
					}
				} else {
					repoCount = len(p.ArchivedWorktrees)
					for _, wt := range p.ArchivedWorktrees {
						repoNames = append(repoNames, wt.RepoName)
					}
				}

				if verbose {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
						p.Project.Name,
						p.Project.Status,
						humanizeTime(p.Project.CreatedAt),
						repoCount,
						strings.Join(repoNames, ", "),
					)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
						p.Project.Name,
						p.Project.Status,
						humanizeTime(p.Project.CreatedAt),
						repoCount,
					)
				}
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show repo names")
	return cmd
}

// humanizeTime returns a human-friendly relative time string.
func humanizeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
