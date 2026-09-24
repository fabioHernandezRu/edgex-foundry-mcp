## ADDED Requirements

### Requirement: Version source of truth
Annotated git tags `vMAJOR.MINOR.PATCH` SHALL be the only source of the release version.
The version SHALL be injected at build time and reported by `--version` and by the MCP
`serverInfo.version`. There SHALL be no VERSION file.

#### Scenario: Tagged build reports the tag version
- **WHEN** a release is built from tag `v0.1.0`
- **THEN** the binary prints `edgex-foundry-mcp 0.1.0` for `--version`

### Requirement: Release artifacts
A release SHALL produce, through GoReleaser:
- static binaries (`CGO_ENABLED=0`, `-trimpath`) for linux/amd64 and linux/arm64,
  packaged as tar.gz archives with `LICENSE` and `README.md`
- a `checksums.txt` (SHA-256)
- a changelog built from conventional commits, grouped into Features (`feat`) and Bug
  fixes (`fix`), with `docs`, `chore`, `test`, `ci` and `style` excluded
- a multi-arch OCI image `ghcr.io/<owner>/edgex-foundry-mcp:<version>` that runs as a
  non-root user on a distroless static base, labeled
  `io.modelcontextprotocol.server.name` with the registry server name

The GoReleaser configuration SHALL pass `goreleaser check`, and CI SHALL run that check.

#### Scenario: Snapshot build
- **WHEN** `make release-snapshot` runs without Docker
- **THEN** `dist/` contains linux amd64 and arm64 tar.gz archives and `checksums.txt`

#### Scenario: Configuration checked in CI
- **WHEN** a pull request changes `.goreleaser.yaml`
- **THEN** CI runs `goreleaser check` and fails on an invalid configuration

### Requirement: Release workflow
A GitHub Actions workflow SHALL run on pushes of `v*` tags only. It SHALL run the test
suite and publish-guard before GoReleaser. It SHALL grant `contents: write` and
`packages: write` only to the release job. `id-token: write` SHALL be granted only to the
registry publish job. GoReleaser SHALL be pinned to an exact version.

#### Scenario: Tag push releases
- **WHEN** tag `v0.1.0` is pushed
- **THEN** tests and publish-guard run, then GoReleaser publishes the GitHub release and the OCI image

### Requirement: MCP registry metadata and publishing
The repository SHALL contain a `server.json` (registry schema 2025-12-11) with:
- name `io.github.<owner>/edgex-foundry-mcp`
- a description of at most 100 characters
- one `oci` package with `stdio` transport, documenting the `EDGEX_*` environment
  variables, with `EDGEX_TOKEN` marked secret

The name SHALL equal the OCI image label. The workflow SHALL set the version and image tag
from the git tag before publishing. Publishing SHALL run only when the repository variable
`MCP_REGISTRY_PUBLISH` is `true`, using GitHub OIDC.

#### Scenario: Metadata consistency
- **WHEN** the repository test suite runs
- **THEN** it asserts that the `server.json` name equals the OCI label in `.goreleaser.yaml`, that the description has at most 100 characters, and that `EDGEX_TOKEN` is marked secret

#### Scenario: Registry publish disabled by default
- **WHEN** a tag is pushed and `MCP_REGISTRY_PUBLISH` is not `true`
- **THEN** the release completes and the registry publish job is skipped
