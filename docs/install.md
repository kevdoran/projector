# Installing pj

## Homebrew (macOS / Linux)

Add the `kevdoran` homebrew tap, then install the `pj` formula:

```bash
brew tap kevdoran/tap  # one-time command to add the kevdoran tap
brew install pj
```

The `pj` formula is a build-from-source formula: Homebrew downloads the source
tarball for the release tag and compiles it with `go build` on your machine
(`go` is pulled in automatically as a build dependency). The `brew tap
kevdoran/tap` shorthand maps to the `kevdoran/homebrew-tap` repository.

To upgrade to the latest version:

```bash
brew upgrade pj
```

### Upgrading from a cask install

Earlier releases distributed `pj` as a Homebrew **cask**. It is now distributed
as a Homebrew **formula**. If you previously installed the cask, migrate with:

```bash
# uninstall the old cask
brew uninstall --cask kevdoran/tap/pj

# install the formula (the tap is the same)
brew tap kevdoran/tap  # one-time command to add the kevdoran tap
brew install kevdoran/tap/pj
```

Once a `tap_migrations.json` mapping the old cask to the new formula is added to
the `kevdoran/homebrew-tap` repo, this migration will happen automatically and
you can simply run `brew upgrade pj`.

## Binary download

Grab the latest release from [GitHub Releases](https://github.com/kevdoran/projector/releases), extract the archive, and move `pj` to a directory on your PATH:

```bash
# Example for macOS arm64
curl -L https://github.com/kevdoran/projector/releases/latest/download/pj_darwin_arm64.tar.gz | tar xz
mv pj /usr/local/bin/pj
```

To upgrade, repeat the same steps — the new binary will overwrite the old one.

## From source

Requires Go 1.25+ and git 2.5+.

```bash
git clone https://github.com/kevdoran/projector.git
cd projector
make build
mv pj /usr/local/bin/pj
```

To upgrade, pull the latest changes and rebuild:

```bash
cd projector
git pull
make build
mv pj /usr/local/bin/pj
```
