# projector

A CLI tool for managing parallel programming projects across multiple git repositories.

`projector` abstracts git worktree management behind a "project" concept. Create a named project and `projector` automatically creates git worktrees in all your configured repositories — no more manual `git worktree add/remove` bookkeeping.

## User Guide

### Installation

**From source** (requires Go 1.22+ and git 2.5+):

```bash
git clone https://github.com/kevdoran/projector.git
cd projector
go build -o projector ./cmd/projector

# Move to a directory on your PATH, e.g.:
mv projector /usr/local/bin/projector
```

### First-time Setup

On first run, `projector` will guide you through interactive setup to configure:

- **Projects directory** — where project directories will be created (e.g. `~/projects`)
- **Repository search directories** — directories to scan for git repositories (e.g. `~/dev/work,~/dev/personal`)

Configuration is saved to `~/.projector/projector-config.toml`.

### Configuration File

`~/.projector/projector-config.toml`:

```toml
projects-dir = "/Users/alice/projects"
template-dir = ""                       # optional: files copied to each new project

repo-search-dirs = [
  "/Users/alice/dev/work",
  "/Users/alice/dev/personal"
]

# Optional: per-repository overrides
[repos.legacy-repo]
default-base = "origin/develop"
```

### Commands

#### `projector list`

List all projects.

```
projector list
projector list --verbose    # also shows repo names
```

Example output:

```
PROJECT    STATUS    CREATED       REPOS
foo        active    2 days ago    4
bar        active    3 weeks ago   2
old-work   archived  1 year ago    5
```

Active projects are listed before archived projects; within each group, projects are sorted alphabetically.

#### `projector create <name> [repos...]`

Create a new project, setting up git worktrees in the specified repositories.

```bash
# Interactive repo selection (multi-select prompt)
projector create my-feature

# Specify repos by name (discovered from search dirs) or absolute path
projector create my-feature repo-a repo-b
projector create my-feature /abs/path/to/some-repo

# Copy repo list from an existing project
projector create new-feature --from existing-feature

# Empty project (add repos later)
projector create my-feature --empty

# Use current branch of each repo as the base (instead of origin/main)
projector create my-feature --current-branch

# Use a template directory
projector create my-feature --template /path/to/template
```

**Branch naming**: `projector` tries `<project-name>` first, then `<project-name>-YYYY-MM-DD`, then `<project-name>-YYYY-MM-DD-1`, `-2`, etc.

**Branch base**: By default branches are created from `origin/main` → `origin/master` → `HEAD` (configurable per-repo via `[repos.<name>]` in the config file). Use `--current-branch` to branch from the repo's current HEAD instead.

**Rollback**: If any worktree fails to be created, all previously created worktrees and the project directory are removed automatically.

#### `projector add-repo [project] [repos...]`

Add one or more repositories to an existing project.

```bash
# From inside a project directory — interactive selection
projector add-repo

# Specify the project and repos
projector add-repo my-feature new-repo

# From inside a project directory — specify repos
projector add-repo new-repo /abs/path/to/another-repo
```

#### `projector archive [project]`

Archive an active project. Removes all git worktrees (reclaims disk space) while keeping the branches in each repository.

```bash
projector archive               # detect project from current directory
projector archive my-feature
```

The `.projector.toml` is updated to `status = "archived"` and the worktree state is saved for future restore. **Uncommitted changes in any worktree will prevent archiving.**

#### `projector restore [project]`

Restore an archived project by recreating all its git worktrees.

```bash
projector restore               # detect project from current directory
projector restore my-feature
```

If a branch no longer exists, a new branch is created with the standard naming strategy. Missing repositories are skipped with a warning.

### Directory Structure

For two repos (`git-repo-1`, `git-repo-2`) and two projects (`foo`, `bar`):

```
~/dev/work/git-repo-1           # original repo clones
~/dev/work/git-repo-2
~/projects/foo/
  .projector.toml               # project metadata
  git-repo-1+foo/               # git worktree (branch: foo)
  git-repo-2+foo/               # git worktree (branch: foo)
~/projects/bar/
  .projector.toml
  git-repo-1+bar/               # git worktree (branch: bar)
  git-repo-2+bar/               # git worktree (branch: bar)
```

---

## Developer Guide

### Prerequisites

- Go 1.22 or later
- git 2.5 or later (first worktree support)

### Build

```bash
go build -o projector ./cmd/projector
```

### Test

```bash
go test -v -race -count=1 ./...
```

Tests use `t.TempDir()` for isolation (auto-cleaned). Integration tests in `internal/git` use real git repositories.

### Vet & Tidy

```bash
go vet ./...
go mod tidy
```

### Project Layout

```
projector/
  cmd/projector/
    main.go          cobra root + subcommand wiring
    list.go
    create.go
    addrepo.go
    archive.go
    restore.go
    resolve.go       shared resolveProject helper
  internal/
    config/          GlobalConfig, Load/Save/ResolveBase
    project/         ProjectConfig, Load/Save/ListAll/DiscoverWorktrees
    git/             RunGit and all git wrappers
    repo/            Discover, ResolveRepos
    tui/             SelectRepos, InitConfig (charmbracelet/huh)
  go.mod
  CLAUDE.md          conventions and build/test commands for coding agents
  README.md
  .github/workflows/
    ci.yml
```

### Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/BurntSushi/toml` | TOML parsing |
| `github.com/charmbracelet/huh` | Interactive terminal forms |
| `github.com/olekukonko/tablewriter` | Table output |

### Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes, add tests.
4. Run `go test -v -race ./...` and `go vet ./...`.
5. Open a pull request against `main`.

CI runs automatically on all pull requests.
