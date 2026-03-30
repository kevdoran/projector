# pj Command Reference

## Configuration

### `pj config setup`

Interactive wizard to configure global settings. Re-running updates existing configuration — previous values are pre-populated.

```bash
pj config setup
```

Sets up:
- **Projects directory** — where project directories are created (e.g. `~/code/projects`)
- **Repository search directories** — directories scanned for git repos (e.g. `~/code/repos/work,~/code/repos/personal`)

Configuration is saved to `~/.projector/projector-config.toml`.

### `pj config list`

Display current configuration in flattened dot-notation format.

```bash
pj config list
```

Output:
```
projects-dir=/Users/alice/projects
repo-search-dirs.0=/Users/alice/repos/work
repo-search-dirs.1=/Users/alice/repos/personal
repos.my-repo.default-base=origin/develop
```

### `pj config get <key>`

Get an individual configuration value.

```bash
pj config get projects-dir
pj config get repo-search-dirs
pj config get repos.my-repo.default-base
```

### `pj config set <key> <value>`

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

### `pj config unset <key>`

Remove or clear an optional configuration value. Required keys (`projects-dir`, `repo-search-dirs`) cannot be unset.

```bash
pj config unset default-editor                   # clear default editor
pj config unset editors.aider                    # remove a custom editor entry
pj config unset repos.my-repo.default-base       # remove per-repo base override
```

---

## Project Commands

### `pj project list`

List projects. By default only active projects are shown.

```bash
pj project list
pj project list --verbose    # also shows repo names
pj project list --all        # include archived projects
```

Output:
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

### `pj project create <name> [repos...]`

Create a new project with git worktrees across the specified repositories.

```bash
# Interactive repo selection
pj project create my-feature

# Specify repos by name or absolute path
pj project create my-feature repo-a repo-b
pj project create my-feature /abs/path/to/some-repo

# Copy repo list from an existing project
pj project create new-feature --from existing-feature

# Empty project (add repos later)
pj project create my-feature --empty

# Branch from a specific ref
pj project create my-feature --base origin/main
pj project create my-feature --base my-other-branch
pj project create my-feature --base v2.3.0
pj project create my-feature --base HEAD
```

**Branch naming**: tries `<project-name>`, then `<project-name>-YYYY-MM-DD`, then `<project-name>-YYYY-MM-DD-1`, `-2`, etc.

**Branch base**: Branches are created from `origin/main` → `HEAD` by default (configurable per-repo via `[repos.<name>]`). Use `--base <ref>` for an explicit ref. With `--from`, each repo branches from the corresponding worktree branch of the source project.

**Checkout existing branch**: `--checkout` with `--base` checks out an existing branch instead of creating a new one. Git DWIM handles both local and remote-tracking branches.

```bash
pj project create my-feature --checkout --base feature-branch
pj project create my-feature --checkout --base origin/feature-branch
```

**Detached HEAD**: `--detached` skips branch creation; worktrees are created in detached HEAD state.

```bash
pj project create my-feature --detached
pj project create my-feature --detached --base origin/release-2.0
```

**Auto-fetch**: When the base ref is a remote-tracking ref (e.g. `origin/main`), `pj` fetches only that ref before creating the worktree — not the entire remote.

**Rollback**: If any worktree fails to be created, all previously created worktrees and the project directory are removed.

### `pj project desc [project]`

Show details for a project. Resolves from the project name or the current directory.

```bash
pj project desc               # detect from current directory
pj project desc my-feature
pj project desc my-feature -v # verbose: full git status per worktree
```

Default output:
```
REPO            BRANCH          STATUS
my-api          feature-x       clean
my-frontend     feature-x       dirty (3)
my-backend      feature-x-2024  clean
```

Verbose output (`-v`):
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
```

### `pj project open [project]`

Open a project in an editor or IDE.

```bash
pj project open               # detect from current directory
pj project open my-feature    # prompts for editor
pj project open my-feature -e cursor   # use specific editor
```

Interactive prompt shows all installed editors:
```
? Choose an editor
> Cursor
  VS Code
  Windsurf
  Zed
  Claude Code
  Finder (macOS)
```

Set a default to skip the prompt:
```bash
pj config set default-editor cursor
```

**Terminal editors** (e.g. Claude Code) print a `cd` + command instead of launching a GUI process:
```
$ pj project open my-feature -e claude
To open in Claude Code, run:

  cd /Users/alice/projects/my-feature && claude
```

**Custom editors** — define additional editors in `~/.projector/projector-config.toml`:
```toml
default-editor = "cursor"

[editors.aider]
name = "Aider"
command = "aider"
terminal = true
```

Custom editors appear in the prompt if their command is on PATH. `terminal = true` causes `pj` to print the `cd` command instead of launching.

**CLI launcher setup**:
- **VS Code**: install `code` from Command Palette → Shell Command: Install
- **Cursor**: install `cursor` from Command Palette
- **IntelliJ IDEA**: Tools → Create Command-line Launcher

### `pj project path [project]`

Print the absolute path to the project directory. Useful for scripting.

```bash
pj project path my-feature
# /Users/alice/projects/my-feature

cd $(pj project path my-feature)
```

### `pj project add-repo [repos...]`

Add one or more repositories to an existing project.

```bash
# From inside a project directory — interactive selection
pj project add-repo

# Specify repos by name or path
pj project add-repo new-repo /abs/path/to/another-repo

# Specify the project explicitly
pj project add-repo my-feature new-repo

# Branch from a specific ref
pj project add-repo --base origin/develop new-repo

# Check out an existing branch
pj project add-repo --checkout --base feature-branch new-repo

# Detached HEAD state
pj project add-repo --detached new-repo
```

### `pj project archive [project]`

Archive an active project. Removes all git worktrees (reclaims disk space) while keeping the branches in each repository.

```bash
pj project archive               # detect from current directory
pj project archive my-feature
```

The `.projector.toml` is updated to `status = "archived"` and worktree state is saved for future restore. **Uncommitted changes in any worktree will prevent archiving.**

### `pj project restore [project]`

Restore an archived project by recreating all its git worktrees.

```bash
pj project restore               # detect from current directory
pj project restore my-feature
```

If a branch no longer exists, a new branch is created with the standard naming strategy. Missing repositories are skipped with a warning.

### `pj project delete [project]`

Permanently delete a project. Removes all git worktrees and the project directory. Works on both active and archived projects.

```bash
pj project delete               # detect from current directory
pj project delete my-feature
pj project delete my-feature --delete-branches   # also delete git branches
pj project delete my-feature --yes               # skip confirmation
```

Before proceeding, the command previews the exact actions and suggests alternatives:
```
The following actions will be performed:

  api:
    git worktree remove /Users/alice/projects/my-feature/api
  frontend:
    git worktree remove /Users/alice/projects/my-feature/frontend

  rm -rf /Users/alice/projects/my-feature

Alternatives:
  - To archive (reversible): pj project archive my-feature

Proceed? [y/N]:
```

**Safety checks** — the command refuses if:
- Any worktree has uncommitted changes or untracked files
- The project directory contains unexpected files
- `--delete-branches` is set and a branch has unpushed commits (override with `--force`)

`--force` bypasses only the unpushed-commits check.

---

## Other Commands

### `pj version`

Show version and build info.

```bash
pj version
pj --version
```

Output:
```
📽️  pj v1.2.3
    commit    abc1234
    built     2026-02-24
    go        go1.22.0
    platform  darwin/arm64
```

---

## Configuration File

`~/.projector/projector-config.toml`:

```toml
projects-dir = "/Users/alice/projects"

repo-search-dirs = [
  "/Users/alice/code/repos/work",
  "/Users/alice/code/repos/personal"
]

# Optional: default editor for "pj project open"
default-editor = "cursor"

# Optional: per-repository base ref overrides
[repos.my-repo]
default-base = "origin/develop"

# Optional: custom editor definitions
[editors.aider]
name = "Aider"
command = "aider"
terminal = true
```

---

## Directory Structure

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
