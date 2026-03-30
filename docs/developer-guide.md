# pj Developer Guide

## Prerequisites

- Go 1.25 or later
- git 2.5 or later (first worktree support)

## Build

```bash
make build    # build ./pj with version info injected from git
make install  # install to $(go env GOPATH)/bin
make test     # go test -v -race -count=1 ./...
make vet      # go vet ./...
make tidy     # go mod tidy
make clean    # remove ./pj
```

`make build` uses `git describe --tags --always --dirty` for the version string. Tagged releases produce `v1.2.3`; development builds show the commit hash with a `-dirty` suffix if there are uncommitted changes.

Tests use `t.TempDir()` for isolation (auto-cleaned). Integration tests in `internal/git` use real git repositories — no mocking.

## Project Layout

```
projector/
  cmd/projector/
    main.go            cobra root + subcommand wiring
    projects.go        "project" noun command
    config.go          "config" noun command
    config_run.go      interactive configuration wizard
    config_list.go     display current configuration
    config_get.go      get config value
    config_set.go      set individual config values
    config_unset.go    unset config values
    list.go
    create.go
    desc.go
    addrepo.go
    archive.go
    restore.go
    delete.go
    open.go
    path.go
    resolve.go         shared resolveProject helper
    version.go
  internal/
    config/            GlobalConfig, EditorConfig, Load/Save/ResolveBase
    project/           ProjectConfig, Load/Save/ListAll/DiscoverWorktrees
    git/               RunGit and all git wrappers
    repo/              Discover, ResolveRepos
    tui/               SelectRepos, SelectEditor, ExpandHome (charmbracelet/huh)
  docs/
  go.mod
  Makefile
  CLAUDE.md            conventions and build/test commands for coding agents
  README.md
  .github/workflows/
    ci.yml
    release.yml
```

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/BurntSushi/toml` | TOML parsing |
| `github.com/charmbracelet/huh` | Interactive terminal forms |

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes and add tests.
4. Run `make test` and `make vet`.
5. Open a pull request against `main`.

CI runs automatically on all pull requests.
