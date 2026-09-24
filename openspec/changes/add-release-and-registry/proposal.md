> **Status: stub (Phase 2).** Only the proposal exists. The `release` skill documents the
> intended procedure and marks it as not yet wired up.

## Why

Users on edge devices (Raspberry Pi 4, linux/arm64) should not need a Go toolchain. MCP
clients can also discover servers through the MCP registry. Both need a reproducible,
versioned release.

## What Changes

- The version source of truth is annotated git tags `vX.Y.Z`. The version is injected with
  `-ldflags -X main.version` and reported by `--version` and MCP `serverInfo.version`.
- `.goreleaser.yaml`: linux/amd64 and linux/arm64, `CGO_ENABLED=0`, tar.gz archives,
  checksums, and a changelog grouped from conventional commits.
- `.github/workflows/release.yml`, triggered by a `v*` tag push, with
  `permissions: contents: write` for that job only.
- `server.json` for the MCP registry, plus the publishing steps.
- Wiring up the `release` skill.

## Non-goals

- Container images, Homebrew or OS packages.
- Windows or macOS binaries (possible later).

## Safety model

No tool changes. The release pipeline runs publish-guard before goreleaser.

## Capabilities

### New Capabilities
- `release`: versioning, artifacts, changelog and registry metadata.

## Impact

New release workflow and goreleaser config, the `release` skill, and README install
instructions.
