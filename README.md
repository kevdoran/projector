# projector | pj

[![CI](https://github.com/kevdoran/projector/actions/workflows/ci.yml/badge.svg)](https://github.com/kevdoran/projector/actions/workflows/ci.yml)

Projector (`pj`) is for managing parallel projects backed by git worktrees.

Create a named project consisting of git worktrees from all the relevant git repositories. The result is a clean, isolated, multi-repo project directory from which you can launch a new Cursor workspace or Claude Code session.

Stop thinking about:

- ❌ Context switching (for you and your agents) using `git stash & git checkout main` or `git commit -a -m WIP && git checkout other-branch`
- ❌ If I start a new agent to work on this bugfix, will it interfere with my current agent working on that big feature?
- ❌ Manually doing Cursor `Add folder to workspace` operations or repetative `git worktree add/remove` bookkeeping

You may have heard that git worktrees are a better solution for working with multiple coding agents in parallel. Maybe you are already using them, but find them tedious to manage, especially when working on a software system that spans mutliple git repositories, and a new parallel task means creating a new worktree in every repo. If so, you may find `pj` a useful tool to abstract "copies" (worktrees) of repositories behind a "project" concept.

## User Guide

### Installation

**From source** (requires Go 1.22+ and git 2.5+):

```bash
brew install go  # if needed

git clone https://github.com/kevdoran/projector.git
cd projector
go build -o pj ./cmd/projector

# Move or copy to a directory on your PATH, e.g.:
mv pj /usr/local/bin/pj
```

### First-time Setup

On first run, `pj` will guide you through interactive setup to configure:

- **Projects directory** — where project directories will be created (e.g. `~/projects`)
- **Repository search directories** — directories to scan for git repositories (e.g. `~/repos/work,~/repos/personal`)

Configuration is saved to `~/.projector/projector-config.toml`.

### Configuration File

`~/.projector/projector-config.toml`:

```toml
projects-dir = "/Users/alice/projects"

repo-search-dirs = [
  "/Users/alice/repos/work",
  "/Users/alice/repos/personal"
]

# Optional: per-repository overrides
[repos.legacy-repo]
default-base = "origin/develop"
```

### Commands

#### `pj version`

Show version and build info.

```
pj version
pj --version
```

Example output:

```
📽️  pj v1.2.3
    commit    abc1234
    built     2026-02-24
    go        go1.22.0
    platform  darwin/arm64
```

Local dev builds (without ldflags) show `dev` / `unknown` for the injected fields.

#### `pj project list`

List all projects.

```
pj project list
pj project list --verbose    # also shows repo names
```

Example output:

```
PROJECT    STATUS    CREATED       REPOS
foo        active    2 days ago    4
bar        active    3 weeks ago   2
old-work   archived  1 year ago    5
```

Active projects are listed before archived projects; within each group, projects are sorted alphabetically.

#### `pj project create <name> [repos...]`

Create a new project, setting up git worktrees in the specified repositories.

```bash
# Interactive repo selection (multi-select prompt)
pj project create my-feature

# Specify repos by name (discovered from search dirs) or absolute path
pj project create my-feature repo-a repo-b
pj project create my-feature /abs/path/to/some-repo

# Copy repo list from an existing project (inherits each repo's current branch as base)
pj project create new-feature --from existing-feature

# Empty project (add repos later)
pj project create my-feature --empty

# Branch from a specific ref (branch, tag, SHA, or remote ref)
pj project create my-feature --base origin/main
pj project create my-feature --base my-other-branch
pj project create my-feature --base v2.3.0
pj project create my-feature --base HEAD
```

**Branch naming**: `pj` tries `<project-name>` first, then `<project-name>-YYYY-MM-DD`, then `<project-name>-YYYY-MM-DD-1`, `-2`, etc.

**Branch base**: By default branches are created from `origin/main` → `HEAD` (configurable per-repo via `[repos.<name>]` in the config file). Use `--base <ref>` to specify any git ref explicitly. When `--from` is used without `--base`, each repo branches from the corresponding worktree branch of the source project.

**Auto-fetch**: When the resolved base ref is a remote-tracking ref (e.g. `origin/main`), `pj` automatically runs `git fetch` for that remote before creating the worktree, so the branch is always created from an up-to-date ref.

**Rollback**: If any worktree fails to be created, all previously created worktrees and the project directory are removed automatically.

#### `pj project open [project]`

Open a project in your configured editor or IDE.

```bash
pj project open               # detect project from current directory
pj project open my-feature
```

The first time this command is run (or when no editor is configured), an interactive prompt lets you choose from the supported options. Installed editors are annotated:

```
? Choose a default editor for 'pj project open'
  Cursor          (installed)
  VS Code         (installed)
  Zed             (not installed)
  Sublime Text    (installed)
  BBEdit          (not installed)
  IntelliJ IDEA   (not installed)
  Finder (macOS)  (installed)
```

The choice is saved to `~/.projector/projector-config.toml`. You can change it there at any time, or set a completely custom command — any executable that accepts a directory path as its first positional argument works:

```toml
# editor: command used by "pj project open". Accepts any executable that takes
# a directory path as its first positional argument (e.g. cursor, code, subl,
# bbedit, idea, zed, finder). You can specify a custom command or script here
# provided it follows the same convention.
editor = "cursor"
```

#### `pj project path [project]`

Print the absolute path to the project directory. Useful for scripting or shell navigation.

```bash
pj project path my-feature
# /Users/alice/projects/my-feature

cd $(pj project path my-feature)
```

#### `pj project add-repo [repos...]`

Add one or more repositories to an existing project.

```bash
# From inside a project directory — interactive selection
pj project add-repo

# From inside a project directory — specify repos
pj project add-repo new-repo /abs/path/to/another-repo

# Specify project explicitly (by name) and repos
pj project add-repo my-feature new-repo
```

#### `pj project desc [project]`

Show details for a project. Resolves from the project name argument or the current directory.

```bash
pj project desc               # detect project from current directory
pj project desc my-feature
pj project desc my-feature -v  # verbose: full git status per worktree
```

Default output — a summary table with one row per worktree:

```
REPO            BRANCH          STATUS
my-api          feature-x       clean
my-frontend     feature-x       dirty (3)
my-backend      feature-x-2024  clean
```

Verbose output (`-v`) — project header followed by a block per worktree, including full `git status --short` lines for dirty repos:

```
Project:  feature-x
Path:     /Users/alice/projects/feature-x
Status:   active
Created:  2 days ago

my-api  [feature-x]
  path    /Users/alice/projects/feature-x/my-api+feature-x
  status  clean

my-frontend  [feature-x]  dirty
  path    /Users/alice/projects/feature-x/my-frontend+feature-x
  status  dirty
     M  src/components/Button.tsx
    ??  src/components/NewComponent.tsx

my-backend  [feature-x-2024]
  path    /Users/alice/projects/feature-x/my-backend+feature-x
  status  clean
```

#### `pj project archive [project]`

Archive an active project. Removes all git worktrees (reclaims disk space) while keeping the branches in each repository.

```bash
pj project archive               # detect project from current directory
pj project archive my-feature
```

The `.projector.toml` is updated to `status = "archived"` and the worktree state is saved for future restore. **Uncommitted changes in any worktree will prevent archiving.**

#### `pj project delete [project]`

Permanently delete a project. Removes all git worktrees and the project directory. Works on both active and archived projects.

```bash
pj project delete               # detect project from current directory
pj project delete my-feature
pj project delete my-feature --delete-branches   # also delete git branches
pj project delete my-feature --yes               # skip confirmation prompt
```

A confirmation prompt is shown before any destructive action. Pass `-y` / `--yes` to skip it (useful in scripts).

**Safety checks** — the following will cause the command to refuse:
- Any worktree has uncommitted changes or untracked files
- The project directory contains unexpected files (anything other than `.projector.toml` and worktree subdirectories)
- `--delete-branches` is set and a branch has unpushed commits (override with `--force`)

`--force` only bypasses the unpushed commits check. Dirty worktrees and unexpected project directory files are always enforced.

#### `pj project restore [project]`

Restore an archived project by recreating all its git worktrees.

```bash
pj project restore               # detect project from current directory
pj project restore my-feature
```

If a branch no longer exists, a new branch is created with the standard naming strategy. Missing repositories are skipped with a warning.

### Directory Structure

For two repos (`git-repo-1`, `git-repo-2`) and two projects (`foo`, `bar`):

```
~/repos/git-repo-1           # original repo clones
~/repos/git-repo-2
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
make build    # build ./pj with version info from git
make install  # install to $(go env GOPATH)/bin
make test     # go test -v -race -count=1 ./...
make vet      # go vet ./...
make tidy     # go mod tidy
make clean    # remove ./pj
```

`make build` uses `git describe --tags --always --dirty` for the version string, so tagged releases produce a clean `v1.2.3` while development builds show the commit hash (with a `-dirty` suffix if there are uncommitted changes).

Tests use `t.TempDir()` for isolation (auto-cleaned). Integration tests in `internal/git` use real git repositories.

### Project Layout

```
projector/
  cmd/projector/
    main.go          cobra root + subcommand wiring
    projects.go      "projects" noun command
    list.go
    create.go
    desc.go
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

### Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes, add tests.
4. Run `go test -v -race ./...` and `go vet ./...`.
5. Open a pull request against `main`.

CI runs automatically on all pull requests.
