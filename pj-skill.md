# pj — Agent Skill File

`pj` manages parallel, isolated project workspaces backed by git worktrees. Each project is a directory containing one worktree per repository, all on a shared branch name.

**When to use `pj`**: when a user asks you to create, open, describe, archive, restore, or delete a project; when you need to work on a feature that spans multiple repositories simultaneously.

## This Skill File

This skill file was introduced in **v1.0.0** — it does not exist in earlier releases. If `pj version` reports a version older than v1.0.0, this file is not available for that version.

Each release of `pj` v1.0.0 and later ships a corresponding version of this skill file. To get the skill file that matches the installed version:

```bash
pj version   # shows installed version, e.g. v1.2.3
```

Then fetch the matching skill file:
```
https://github.com/kevdoran/projector/releases/download/<version>/pj-skill.md
```

The latest version is always at:
```
https://github.com/kevdoran/projector/releases/latest/download/pj-skill.md
```

**If a feature described by the user is not covered in this file**, check whether it exists in the latest skill file. If it does, the installed version of `pj` is outdated — prompt the user to upgrade:

```bash
brew upgrade pj   # if installed via Homebrew
# or: download the latest binary from https://github.com/kevdoran/projector/releases/latest
```

## Core Concepts

- **Project**: a named directory (`<projects-dir>/<name>/`) containing one git worktree per repository
- **Worktree**: a linked working tree from a git repository, checked out at a branch named after the project
- **Projects directory**: configured at `~/.projector/projector-config.toml` (`projects-dir` key)

## Essential Commands

### Check configuration
```bash
pj config list        # show all config values
pj config get projects-dir
pj config get repo-search-dirs
```

### First-time setup (if not configured)
```bash
pj config setup       # interactive wizard — sets projects-dir and repo-search-dirs
```

### List projects
```bash
pj project list           # active projects only
pj project list --all     # include archived
pj project list --verbose # also show repo names
```

### Create a project
```bash
# Interactive repo selection
pj project create my-feature

# Specify repos explicitly (by name or absolute path)
pj project create my-feature api frontend infra

# Branch from a specific ref
pj project create my-feature --base origin/develop

# Check out an existing branch
pj project create my-feature --checkout --base existing-branch

# Copy repos from an existing project
pj project create new-feature --from existing-feature

# Empty project (add repos later)
pj project create my-feature --empty

# Detached HEAD (no branch created)
pj project create my-feature --detached
```

### Describe a project
```bash
pj project desc my-feature      # summary table: repo, branch, status
pj project desc my-feature -v   # verbose: full git status per worktree
pj project desc                  # auto-detect from current directory
```

Example output:
```
REPO        BRANCH       STATUS
api         my-feature   clean
frontend    my-feature   dirty (2)
```

### Get the project path
```bash
pj project path my-feature       # prints absolute path
cd $(pj project path my-feature) # navigate to it
```

### Open a project in an editor
```bash
pj project open my-feature             # interactive editor selection
pj project open my-feature -e cursor   # specific editor
pj project open my-feature -e code     # VS Code
pj project open my-feature -e claude   # prints cd + claude command (terminal editor)
pj project open                        # auto-detect from current directory
```

For terminal editors (e.g. `claude`, `aider`), `pj project open` prints a `cd` + command to run rather than launching directly. Execute the printed command.

### Add a repo to an existing project
```bash
pj project add-repo my-feature new-repo
pj project add-repo my-feature --base origin/develop new-repo
pj project add-repo my-feature --checkout --base feature-branch new-repo
```

### Archive a project (removes worktrees, keeps branches)
```bash
pj project archive my-feature
```
Fails if any worktree has uncommitted changes. Saves worktree state for restore.

### Restore an archived project
```bash
pj project restore my-feature
```

### Delete a project (irreversible — only when explicitly requested)
**Do not run `pj project delete` unless the user explicitly asks to delete the project.** Prefer `pj project archive` for all routine cleanup — it is reversible and the safe default.

```bash
pj project delete my-feature          # shows preview, prompts for confirmation
pj project delete my-feature --yes    # skip prompt
pj project delete my-feature --delete-branches   # also delete git branches
```
Fails if any worktree has uncommitted changes, untracked files, or the project directory has unexpected files.

## Common Workflows

### Start work on a new feature
```bash
pj project create my-feature api frontend    # creates worktrees on branch "my-feature"
pj project path my-feature                   # get the path
# Each subdirectory is a worktree — cd into one and commit normally
```

### Check status across all repos
```bash
pj project desc my-feature
```

### Clean up after merging (default: archive — do not delete unless user explicitly asks)
```bash
pj project archive my-feature
```

### Continue working on a previously archived project (e.g., to fix something)
```bash
pj project restore my-feature
```

## Configuration Keys

| Key | Description |
|-----|-------------|
| `projects-dir` | Root directory for all project directories |
| `repo-search-dirs` | Directories scanned for git repositories |
| `default-editor` | Editor used by `pj project open` without prompting |
| `repos.<name>.default-base` | Per-repo default base ref (overrides `origin/main`) |
| `editors.<name>.command` | Custom editor executable |
| `editors.<name>.terminal` | `true` if editor runs in terminal (prints cd command) |

```bash
pj config set projects-dir ~/projects
pj config set repo-search-dirs /path1,/path2
pj config set --add repo-search-dirs /additional/path
pj config set --remove repo-search-dirs /old/path
pj config set default-editor cursor
pj config set repos.my-repo.default-base origin/develop
pj config set editors.aider.command aider
pj config set editors.aider.terminal true
```

## Error Handling Notes

- **Not configured**: if `pj config list` fails or shows no `projects-dir`, run `pj config setup` first
- **Dirty worktrees**: `archive` and `delete` refuse to proceed; commit or stash changes first
- **Missing ref**: `--base <ref>` validates the ref exists across repos before creating worktrees; if absent in some repos you'll be prompted whether to proceed with fallback bases
- **Branch conflicts**: `pj` tries `<name>`, then `<name>-YYYY-MM-DD`, then `<name>-YYYY-MM-DD-N` to avoid collisions
