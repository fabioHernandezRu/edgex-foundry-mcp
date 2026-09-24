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
- A multi-arch OCI image (`ghcr.io/<owner>/edgex-foundry-mcp`, linux/amd64 and
  linux/arm64), because the MCP registry has no package type for plain binaries. The image
  carries the ownership label the registry requires.
- `server.json` for the MCP registry, published by the release workflow with
  `mcp-publisher`. The publish step is gated behind an explicit repository variable,
  because the ghcr.io package must be made public first.
- Wiring up the `release` skill.

## Non-goals

- Homebrew or OS packages, and `.mcpb` bundles (they have no per-architecture field).
- Windows or macOS binaries (possible later).

## Safety model

No tool changes. The release pipeline runs publish-guard before goreleaser.

## Capabilities

### New Capabilities
- `release`: versioning, artifacts, changelog and registry metadata.

## Impact

New release workflow and goreleaser config, the `release` skill, and README install
instructions.
