# Installing pj

## Homebrew (macOS / Linux)

Add the `kevdoran` homebrew tap, then install the `pj` formula:

```bash
brew tap kevdoran/tap  # one-time command to add the kevdoran tap
brew install pj
```

To upgrade to the lastest version:

```bash
brew upgrade pj
```

### Upgrading from v0.x (cask install)

Prior to v1.0.0, `pj` was distributed as a Homebrew cask. To migrate to the Homebrew formula:

```bash
# uninstall from homebrew's cask directory
brew uninstall --cask kevdoran/tap/pj

# install to homebrew's formula directory
brew tap kevdoran/tap  # one-time command to add the kevdoran tap
brew install pj
```

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
