# Git Worktrees: Parallel Workspaces Without the Mess

## What are git worktrees?

A git worktree is an additional working copy of a repository that shares the same `.git` history. Instead of cloning a repo multiple times or constantly switching branches, you can have multiple branches checked out simultaneously in separate directories — each with its own working tree, but all backed by a single repository.

```bash
# Create a worktree for a new branch
git worktree add ../my-repo-feature-x -b feature-x

# Now you have two working directories:
#   my-repo/              (main branch)
#   my-repo-feature-x/    (feature-x branch)
```

Both directories are real, independent working copies. You can build, test, and edit in one without affecting the other. No stashing, no juggling uncommitted changes.

## Why this matters for coding agents

When you run a coding agent (Cursor, Claude Code, Copilot, etc.), it operates in a workspace — a directory on disk. If you and an agent are sharing the same checkout, or two agents are working on different tasks in the same checkout, things break:

- Agents overwrite each other's changes
- You can't review one agent's work while another is running
- A `git stash && git checkout` in one terminal disrupts everything else

Worktrees solve this by giving each task its own directory:

```
~/repos/my-api/                    # original clone (main branch)

~/projects/feature-x/
    my-api+feature-x/              # worktree (feature-x branch)

~/projects/bugfix-y/
    my-api+bugfix-y/               # worktree (bugfix-y branch)
```

Each agent gets its own isolated workspace. No conflicts, no coordination needed. You can spin up a new task in seconds and tear it down when you're done — the branch stays in the original repo.

## The multi-repo problem

Worktrees work great for a single repo. But real projects often span multiple repositories — a backend API, a frontend app, shared libraries, infrastructure config. Setting up a parallel workspace means running `git worktree add` in every repo, choosing branch names, organizing the directories. Tearing it down means running `git worktree remove` in each one.

This is the problem `pj` solves:

```
Without pj                              With pj
──────────────────────────              ──────────────────────────

$ cd ~/repos/api                        $ pj project create feature-x
$ git worktree add \                    # Done. All worktrees created,
    ~/projects/feature-x/api \          # directory organized, ready
    -b feature-x                        # to open in your editor.
$ cd ~/repos/frontend
$ git worktree add \
    ~/projects/feature-x/frontend \
    -b feature-x
$ cd ~/repos/infra
$ git worktree add \
    ~/projects/feature-x/infra \
    -b feature-x
# ... repeat for each repo
```

The result is a clean project directory you can open as a single workspace:

```
~/projects/feature-x/
    .projector.toml
    api+feature-x/           # worktree of api repo
    frontend+feature-x/      # worktree of frontend repo
    infra+feature-x/         # worktree of infra repo

~/projects/bugfix-y/
    .projector.toml
    api+bugfix-y/
    frontend+bugfix-y/
```

Each project is a self-contained workspace. Open it in Cursor, point Claude Code at it, or work in it yourself — all without touching any other project.

## Key benefits

**No copies, no clones.** Worktrees share the same git history and object store. Creating one is near-instant and uses minimal disk space.

**Full isolation.** Each worktree has its own working tree and index. Builds, uncommitted changes, and editor state are completely independent.

**Branches stay in one place.** All branches live in the original repository. Worktrees are just views into them. When you remove a worktree, the branch remains.

**Scales to any number of parallel tasks.** Need three agents working on three different features across five repos? That's just three `pj project create` commands.

## Further reading

- [git-worktree documentation](https://git-scm.com/docs/git-worktree)
- [pj README](../README.md) — full command reference
