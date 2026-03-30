# 📽️ projector | pj

[![CI](https://github.com/kevdoran/projector/actions/workflows/ci.yml/badge.svg)](https://github.com/kevdoran/projector/actions/workflows/ci.yml)

`pj` manages parallel, isolated workspaces across multiple git repositories. One command creates a [git worktree](docs/worktrees.md) for each of your repos, organized into a project directory you can open in Cursor, VS Code, or hand off to a coding agent.

```bash
pj project create feature-x api frontend infra
# ~/projects/feature-x/
#   api/        ← worktree of api repo, branch: feature-x
#   frontend/   ← worktree of frontend repo, branch: feature-x
#   infra/      ← worktree of infra repo, branch: feature-x
```

`pj` simplifies running multiple agents to work on different tasks simultaneously. No extra git clones, no stashing, no branch switching — just isolated workspaces that share the same git repos and history. `pj` replaces manual `git worktree add/remove` bookkeeping with a simple project abstraction. See [Why git worktrees?](docs/worktrees.md) for background on the approach.

## Install

```bash
brew tap kevdoran/tap  # one-time command to add the kevdoran tap
brew install pj
```

Other methods: [binary download, from source](docs/install.md)

## Quick Start

```bash
# One-time setup: set your projects dir and repo search paths
pj config setup

# Create a project (interactive repo picker if you don't name repos)
pj project create my-feature

# See what's in a project
pj project desc my-feature

# Open it in your editor
pj project open my-feature
```

## Core Commands

| Command | What it does |
|---|---|
| `pj project create <name> [repos...]` | Create a project with worktrees across repos |
| `pj project list` | List all projects |
| `pj project desc [project]` | Show repo/branch/status for a project |
| `pj project open [project]` | Open in an editor or IDE |
| `pj project path [project]` | Print the project directory path |
| `pj project add-repo [repos...]` | Add repos to an existing project |
| `pj project archive [project]` | Remove worktrees, keep branches |
| `pj project restore [project]` | Recreate worktrees from an archived project |
| `pj project delete [project]` | Permanently delete a project |
| `pj config setup` | Interactive configuration wizard |

Most commands auto-detect the current project if you're inside a project directory, so you can skip the name argument.

## Using with Coding Agents

`pj` ships a skill file that teaches coding agents how to use the CLI.

**Claude Code** — add to your project's `CLAUDE.md`:
```markdown
## pj
Read the pj skill file for instructions on managing projects:
https://github.com/kevdoran/projector/releases/latest/download/pj-skill.md
```

**Cursor** — create `.cursor/rules/pj.mdc`:
```markdown
---
description: Instructions for managing projects with the pj CLI tool
alwaysApply: true
---

When using the pj CLI tool, first fetch and read the skill file at:
https://github.com/kevdoran/projector/releases/latest/download/pj-skill.md
```

## Docs

- [Full command reference](docs/commands.md) — all flags, options, and output examples
- [Why git worktrees?](docs/worktrees.md) — background on the approach
- [Install guide](docs/install.md) — binary download and building from source
- [Developer guide](docs/developer-guide.md) — build, project layout, contributing
