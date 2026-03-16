# CLAUDE.md — Coding Agent Conventions for pj

## Quick Reference

```bash
# Build (preferred — injects version info from git)
make build

# Run tests (all packages)
make test

# Vet
make vet

# Tidy dependencies
make tidy

# Run a single package (no make target — use go directly)
go test -v -race -count=1 ./internal/git/...
```

## Package Overview

| Package | Responsibility |
|---|---|
| `cmd/projector` | Cobra root + one file per subcommand (`projects.go`, `list.go`, `create.go`, `desc.go`, `open.go`, `path.go`, `addrepo.go`, `archive.go`, `restore.go`, `delete.go`, `version.go`, `config.go`, `config_run.go`, `config_list.go`, `config_get.go`, `config_set.go`, `config_unset.go`). No business logic — delegate to internal packages. |
| `internal/config` | `GlobalConfig` struct, `EditorConfig` struct, `Load`/`Save`/`ResolveBase`/`Validate`. TOML I/O for `~/.projector/projector-config.toml`. |
| `internal/project` | `ProjectConfig` struct, `Load`/`Save`/`ListAll`/`FindProjectDir`/`ValidateName`/`DiscoverWorktrees`. TOML I/O for `<projects-dir>/<name>/.projector.toml`. |
| `internal/git` | Thin wrappers around the `git` executable: `RunGit`, `WorktreeAdd`, `WorktreeAddDetached`, `WorktreeRemove`, `WorktreeList`, `WorktreeForBranch`, `StatusPorcelain`, `RefExists`, `BranchExists`, `BranchCheckedOut`, `CurrentBranch`, `AvailableBranchName`, `BranchNameFromRef`, `Remotes`, `RemoteForRef`, `Fetch`, `FetchRef`, `HasUnpushedCommits`, `HeadSHA`, `MinVersionCheck`. |
| `internal/repo` | `Repo` struct, `Discover` (non-recursive scan of search dirs), `ResolveRepos` (name or abs-path lookup). |
| `internal/tui` | `SelectRepos` (huh multi-select), `SelectEditor` (huh single-select, installed editors only), `EditorOption` (with `Terminal` field), `ExpandHome` (tilde expansion). |

## Conventions

### TOML Keys
All TOML keys are **kebab-case**: `projects-dir`, `repo-search-dirs`, `created-at`, `archived-at`, `default-base`, `repo-name`, `repo-path`, `worktree-path`, `archived-worktrees`.

### Error Wrapping
Always wrap errors with `%w` so callers can use `errors.Is`/`errors.As`:
```go
return fmt.Errorf("load project: %w", err)
```

### Sentinel Errors
Each package exposes its own sentinel errors:
- `config.ErrNotFound`
- `project.ErrNotFound`
- `git.ErrDirtyWorktree` (reserved; `StatusPorcelain` returns a bool instead)

### No Business Logic in `main.go`
`cmd/projector/main.go` only wires up cobra. Each subcommand lives in its own file (`list.go`, `create.go`, etc.). All actual logic delegates to `internal/` packages.

### Git Invocation
All git operations go through `git.RunGit(workingDir, args...)`. Never shell out to git directly from other packages — use the `git` package wrappers.

### Import Rules
- `internal/git` imports nothing from this module (only stdlib + no projector packages).
- `internal/config` imports nothing from this module except stdlib.
- `internal/repo` imports nothing from this module.
- `internal/project` may import `internal/git` only for its minimal `gitCurrentBranch` helper (uses `os/exec` directly to avoid circular deps — see inline comment).
- `internal/tui` imports `internal/repo`.
- `cmd/projector` imports everything.

### Tests
- All tests use `t.TempDir()` (auto-cleaned by the test runner).
- Integration tests in `internal/git` use real git repos created with a `createTestRepo(t)` helper.
- No mocking of the git binary.
- No global state mutation in tests (use `t.Setenv` to override `HOME`).

### Worktree Naming Convention
Worktree directories are named after the original git repository directory, e.g. `my-repo`.

### Branch Name Strategy
`AvailableBranchName` tries in order:
1. `<project-name>`
2. `<project-name>-YYYY-MM-DD`
3. `<project-name>-YYYY-MM-DD-1`, `-2`, …

### Dynamic Worktree Discovery
Active project worktree state is **never stored in TOML** — it is discovered at runtime by reading `.git` files in project subdirectories. TOML only stores worktree state when a project is **archived**.

## Definition of Done

Every functional change to the CLI must include:
- **Clean build** — `go build -o pj ./cmd/projector` and `go vet ./...` must pass with no errors
- **Tests** — new or updated tests covering the changed behaviour; `go test -v -race -count=1 ./...` must pass
- **README.md** — user-facing docs updated (commands, flags, output examples)
- **CLAUDE.md** — package/function lists updated if packages or public functions changed

## Minimum Requirements
- Go 1.22+
- git 2.5+ (first worktree support; enforced by `git.MinVersionCheck()`)
