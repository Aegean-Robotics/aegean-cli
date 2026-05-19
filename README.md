# aegean — Aegean Cloud Engine CLI

Single static binary for managing Aegean accounts, API keys, custom domains, sending email/SMS/voice, and deploying static sites from the command line.

Releases at https://github.com/Aegean-Robotics/aegean-cli/releases. See [`todo/aegean-cli-release-plan.md`](../../todo/aegean-cli-release-plan.md) for the release-plan delta.

## Install

```bash
# Debian / Ubuntu — apt repo at api.aegeanengine.com/apt/
echo 'deb [trusted=yes] https://api.aegeanengine.com/apt stable main' \
    | sudo tee /etc/apt/sources.list.d/aegean.list
sudo apt update && sudo apt install aegean-cli

# macOS / Linuxbrew — Homebrew tap
brew install Aegean-Robotics/tap/aegean

# Linux / macOS — curl installer (signed tarball)
curl -fsSL https://aegeanengine.com/install.sh | sh

# Windows — Scoop (v0.2)
scoop install aegean

# Direct .deb / .rpm / .tar.gz / .zip from GitHub Releases
#   https://github.com/Aegean-Robotics/aegean-cli/releases
```

`[trusted=yes]` on the apt source is the v1 compromise — the repo isn't GPG-signed yet (Phase 2 of `todo/aegean-cli-release-plan.md`). Traffic is HTTPS so an attacker would still need to break TLS.

## Local build

```bash
cd apps/aegeanengine/src/cli
go build -o aegean ./cmd/aegean
./aegean version
```

Or via the existing pinned Docker toolchain:

```bash
docker run --rm -v "$(pwd)":/w -w /w/apps/aegeanengine/src/cli \
  golang:1.22 go build -o aegean ./cmd/aegean
```

## Module layout

```
src/cli/
├── cmd/aegean/main.go     # entry point — wires the cobra root
├── internal/
│   ├── client/            # thin wrapper over src/sdk/go (vendored via replace in dev)
│   ├── config/            # ~/.aegean/{config,credentials} TOML loader/writer
│   ├── output/            # text/json/yaml formatters
│   └── commands/          # one file per cobra subcommand
├── VERSION                # 0.1.0 — independent of src/sdk/VERSION
└── .goreleaser.yaml       # release artifacts (deb/rpm/brew/scoop)
```

## Configuration

```
~/.aegean/
├── config         # 0644 — endpoint, default_profile, output, telemetry
└── credentials    # 0600 — JWT, refresh token, account alias
```

Precedence (highest wins): CLI flag → `AEGEAN_*` env var → profile file → built-in default.
