# 📽️ projector | pj

[![CI](https://github.com/kevdoran/projector/actions/workflows/ci.yml/badge.svg)](https://github.com/kevdoran/projector/actions/workflows/ci.yml)

`pj` manages parallel, isolated project workspaces backed by [git worktrees](docs/worktrees.md). A single command creates worktrees across all your repositories, organized into a project directory you can open as a Cursor workspace or point a coding agent at.

```bash
pj project create feature-x api frontend infra
# Creates ~/projects/feature-x/ with a worktree from each repo, ready to go.
```

If you work across multiple repos and run multiple coding agents (or just multiple tasks) in parallel, `pj` replaces the manual `git worktree add/remove` bookkeeping with a simple project abstraction. See [Why git worktrees?](docs/worktrees.md) for background on the approach.

## User Guide

### Installation

**Homebrew** (macOS and Linux):

```bash
brew install --cask kevdoran/tap/pj
```

**Binary download**: grab the latest release from [GitHub Releases](https://github.com/kevdoran/projector/releases), extract, and move `pj` to a directory on your PATH.

**From source** (requires Go 1.25+ and git 2.5+):

```bash
git clone https://github.com/kevdoran/projector.git
cd projector
make build
# Move or copy to a directory on your PATH, e.g.:
mv pj /usr/local/bin/pj
```

### Quick Start

```bash
# First-time setup
$ pj config setup

  # Follow interactive wizard instructions to configure projects directory and repo search paths

  Configuration saved to ~/.projector/projector-config.toml

# Create a project with worktrees across your repos
$ pj project create my-feature

  Select repositories to include in this project:
    ✓ api (~/code/repos/api)
    • frontend (~/code/repos/frontend)
    ✓ docs (~/code/repos/docs)
  > ✓ infra (~/code/repos/infra)

  Fetching origin/main for api... done
  Fetching origin/main for frontend... done
  Fetching origin/main for infra... done
  Created project "my-feature" with 3 repos

$ pj project desc my-feature

  REPO        BRANCH       STATUS
  api         my-feature   clean
  frontend    my-feature   clean
  infra       my-feature   clean

$ pj project open my-feature

  Choose an editor:
  > Cursor
    VS Code
    Zed
```

### First-time Setup

Run `pj config setup` to configure:

- **Projects directory** — where project directories will be created (e.g. `~/code/projects`)
- **Repository search directories** — directories to scan for git repositories (e.g. `~/code/repos/work,~/code/repos/personal`)

If you run any project command before configuring, `pj` will prompt you to run `pj config setup` first.

Configuration is saved to `~/.projector/projector-config.toml`.

### Configuration File

`~/.projector/projector-config.toml`:

```toml
projects-dir = "/Users/alice/projects"

repo-search-dirs = [
  "/Users/alice/code/repos/work",
  "/Users/alice/code/repos/personal"
]

# Optional: per-repository overrides
[repos.unique-repo]
default-base = "origin/develop"
```

### Commands

#### `pj config setup`

Interactive configuration wizard. Sets up or reconfigures global settings (projects directory, repo search directories). Re-running updates existing configuration — previous values are pre-populated in the form.

```bash
pj config setup
```

#### `pj config list`

Display current configuration in flattened dot-notation format.

```bash
pj config list
```

Example output:

```
projects-dir=/Users/alice/projects
repo-search-dirs.0=/Users/alice/repos/work
repo-search-dirs.1=/Users/alice/repos/personal
repos.my-repo.default-base=origin/develop
```

#### `pj config get <key>`

Get an individual configuration value.

```bash
pj config get projects-dir
pj config get repo-search-dirs
pj config get repos.my-repo.default-base
```

#### `pj config set <key> <value>`

Set an individual configuration value.

```bash
pj config set projects-dir ~/new-projects
pj config set default-editor cursor
pj config set repo-search-dirs /path1,/path2            # replaces entire list
pj config set --add repo-search-dirs /additional/path   # appends to list
pj config set --remove repo-search-dirs /old/path       # removes from list
pj config set repos.my-repo.default-base origin/develop
pj config set editors.aider.command aider               # custom editor
pj config set editors.aider.terminal true               # terminal-mode editor
```

#### `pj config unset <key>`

Remove or clear an optional configuration value. Required keys (`projects-dir`, `repo-search-dirs`) cannot be unset.

```bash
pj config unset default-editor                   # clear default editor
pj config unset editors.aider                     # remove a custom editor entry
pj config unset repos.my-repo.default-base        # remove per-repo base override
```

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

List projects. By default only active projects are shown.

```
pj project list
pj project list --verbose    # also shows repo names
pj project list --all        # include archived projects
```

Example output:

```
PROJECT    STATUS    CREATED       REPOS
foo        active    2 days ago    4
bar        active    3 weeks ago   2
```

With `--all`:

```
PROJECT    STATUS    CREATED       REPOS
foo        active    2 days ago    4
bar        active    3 weeks ago   2
old-work   archived  1 year ago    5
```

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

**Checkout existing branch**: Use `--checkout` with `--base` to check out an existing branch instead of creating a new one. Git's DWIM behavior handles both local branches and remote-tracking branches (automatically creating a local tracking branch).

```bash
pj project create my-feature --checkout --base feature-branch
pj project create my-feature --checkout --base origin/feature-branch
```

**Detached HEAD**: Use `--detached` to skip branch creation entirely. Worktrees are created in detached HEAD state, letting you decide later whether and what to name a branch. Works with all other flags (`--base`, `--from`, etc.).

```bash
pj project create my-feature --detached
pj project create my-feature --detached --base origin/release-2.0
```

**Base ref validation**: When `--base` is specified, `pj` checks that the ref exists in each repository before creating worktrees. If the ref is missing from some repos, you're prompted to confirm whether to proceed with fallback bases for those repos. If the ref is missing from all repos, the command aborts with an error.

**Auto-fetch**: When the resolved base ref is a remote-tracking ref (e.g. `origin/main`), `pj` automatically fetches only that ref before creating the worktree, so the branch is always created from an up-to-date ref. Only the needed ref is fetched (not the entire remote), which is significantly faster for large repositories.

**Rollback**: If any worktree fails to be created, all previously created worktrees and the project directory are removed automatically.

#### `pj project open [project]`

Open a project in an editor or IDE.

```bash
pj project open               # detect project from current directory
pj project open my-feature    # prompts for editor each time
pj project open my-feature -e cursor   # use specific editor, no prompt
```

By default, an interactive prompt shows all **installed** editors each time:

```
? Choose an editor
> Cursor
  VS Code
  Windsurf
  Zed
  Claude Code
  Finder (macOS)
```

To skip the prompt, set a default editor:

```bash
pj config set default-editor cursor
```

Or use the `-e` / `--editor` flag for one-off use:

```bash
pj project open my-feature -e code
```

**Terminal editors** — tools like Claude Code that run in the terminal print a `cd` + command instead of launching a GUI process:

```
$ pj project open my-feature -e claude
To open in Claude Code, run:

  cd /Users/alice/projects/my-feature && claude
```

**Custom editors** — define additional editors in the config file under `[editors.<name>]`:

```toml
# default-editor: editor command used by "pj project open" without prompting.
# When set, skips the interactive editor selection. Accepts any executable that
# takes a directory path as its first argument (e.g. cursor, code, subl, zed).
# Remove or leave empty to be prompted each time.
default-editor = "cursor"

[editors.myeditor]
name = "My Custom Editor"
command = "myedit"

[editors.aider]
name = "Aider"
command = "aider"
terminal = true
```

Custom editors appear in the selection prompt if their command is found on PATH. Setting `terminal = true` causes `pj` to print a `cd` + command instead of launching.

**CLI launcher setup** — some editors require setup to be launchable from the command line:

- **IntelliJ IDEA**: Tools → Create Command-line Launcher (creates the `idea` command)
- **VS Code**: install the `code` command from the Command Palette (Shell Command: Install)
- **Cursor**: install the `cursor` command from the Command Palette

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

# Branch from a specific ref
pj project add-repo --base origin/develop new-repo

# Check out an existing branch (requires --base)
pj project add-repo --checkout --base feature-branch new-repo

# Add repos in detached HEAD state (no branch created)
pj project add-repo --detached new-repo
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
  path    /Users/alice/projects/feature-x/my-api
  status  clean

my-frontend  [feature-x]  dirty
  path    /Users/alice/projects/feature-x/my-frontend
  status  dirty
     M  src/components/Button.tsx
    ??  src/components/NewComponent.tsx

my-backend  [feature-x-2024]
  path    /Users/alice/projects/feature-x/my-backend
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

Before proceeding, the command previews the exact git commands it will run and suggests alternatives:

```
The following actions will be performed:

  api:
    git worktree remove /Users/alice/projects/my-feature/api
  frontend:
    git worktree remove /Users/alice/projects/my-feature/frontend

  rm -rf /Users/alice/projects/my-feature

Alternatives:
  - To archive (reversible): pj project archive my-feature
  - To clean up manually, remove the worktrees and directory listed above

Proceed? [y/N]:
```

With `--delete-branches`, the preview also shows the branch deletion commands. Pass `-y` / `--yes` to skip the preview and prompt (useful in scripts); progress output still prints.

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
  git-repo-1/                   # git worktree (branch: foo)
  git-repo-2/                   # git worktree (branch: foo)
~/projects/bar/
  .projector.toml
  git-repo-1/                   # git worktree (branch: bar)
  git-repo-2/                   # git worktree (branch: bar)
```

---

## Developer Guide

### Prerequisites

- Go 1.25 or later
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
    projects.go      "project" noun command
    config.go        "config" noun command
    config_run.go    interactive configuration wizard
    config_list.go   display current configuration
    config_set.go    set individual config values
    list.go
    create.go
    desc.go
    addrepo.go
    archive.go
    restore.go
    resolve.go       shared resolveProject helper
  internal/
    config/          GlobalConfig, EditorConfig, Load/Save/ResolveBase
    project/         ProjectConfig, Load/Save/ListAll/DiscoverWorktrees
    git/             RunGit and all git wrappers
    repo/            Discover, ResolveRepos
    tui/             SelectRepos, SelectEditor, ExpandHome (charmbracelet/huh)
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
