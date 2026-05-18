# aegean — Aegean Cloud Engine CLI

Single static binary for managing Aegean accounts, API keys, custom domains, and sending email/SMS/voice from the command line.

Status: **scaffold only** (v0.1.0 in flight). See [`todo/aegean-cli-release-plan.md`](../../todo/aegean-cli-release-plan.md) for the full release plan.

## Install (planned channels)

```bash
brew install Aegean-Robotics/tap/aegean              # macOS — week 2
curl -fsSL https://aegeanengine.com/install.sh | sh   # Linux/macOS — week 1
sudo apt install aegean-cli                       # Debian/Ubuntu — week 4
npm install -g @aegeanengine/cli                  # any Node host — week 3
scoop install aegean                              # Windows — v0.2
```

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
