## Context

The binary is built from source today. Edge users (Raspberry Pi 4, linux/arm64) need
prebuilt artifacts, and MCP clients discover servers through the MCP registry. The `release`
skill documents a plan that was not wired up. This change wires it up, with verified
tooling versions.

## Goals / Non-Goals

**Goals:** tag-driven releases with binaries, checksums, a changelog and a multi-arch OCI
image; registry metadata in the repo; an opt-in registry publish; the `release` skill made
operational.

**Non-Goals:** Homebrew, OS packages, `.mcpb` bundles, Windows or macOS builds, and
signing or SBOMs (a later change).

## Decisions

### D1. GoReleaser v2.18.2, pinned
- v2.18.2 is the latest stable v2 tag (`git ls-remote` on goreleaser/goreleaser).
- The workflow pins `version: v2.18.2` rather than `~> v2`, for reproducible releases.
- The config uses current field names: `archives[].formats` and `archives[].ids`. The
  singular `format` and `builds` are deprecated since v2.6.

### D2. Changelog regex accepts hyphenated scopes
The documented `\([[:word:]]+\)` pattern does not match `feat(publish-guard):`, so the
groups use `\([^)]+\)`. The scout verified this against a copy of the repo with test
tags: Features and Bug fixes were grouped, and docs/chore/test were excluded.

### D3. The registry needs an OCI image, not binaries
- The registry package types are `npm`, `pypi`, `nuget`, `cargo`, `oci` and `mcpb`. There
  is no type for plain binaries.
- `mcpb` bundles have no CPU architecture field and GoReleaser cannot build them.
- So the release also builds `ghcr.io/<owner>/edgex-foundry-mcp` with GoReleaser
  `dockers_v2`:
  - platforms linux/amd64 and linux/arm64, on `gcr.io/distroless/static-debian12:nonroot`
  - the label `io.modelcontextprotocol.server.name`, which the registry reads anonymously
    to verify ownership
- OCI references must be lowercase, while the registry name is case-sensitive and must
  match the GitHub login exactly (`io.github.fabioHernandezRu/...`).

### D4. Hand-written server.json, published by mcp-publisher
- GoReleaser's `mcp:` pipe is experimental and cannot carry `environmentVariables`, which
  users need in order to configure the server. So `server.json` is committed and published
  with `mcp-publisher` using GitHub OIDC.
- The workflow rewrites `version` and the image tag from the git tag using `jq`.
- The publish job runs only when the repo variable `MCP_REGISTRY_PUBLISH == 'true'`,
  because ghcr.io packages start private and the registry's anonymous label check would
  fail before the owner makes the package public.

### D5. Guardrails
- The release job runs `make test` and `scripts/publish_guard.sh --ci --files-only`
  before GoReleaser.
- Permissions are least privilege per job.
- CI gains a `goreleaser check` step.
- A Go test (`scripts/release_test.go`) keeps `server.json` consistent with
  `.goreleaser.yaml`.

### D6. Repository name
The rename to `edgex-foundry-mcp` is pending. GoReleaser takes the release owner and name
from the git remote rather than hard-coded values, so releases work before and after the
rename. `server.json` and the image use the final name `edgex-foundry-mcp`.

## Risks / Trade-offs

- [The Docker image build could not be run in the authoring environment, which has no
  Docker daemon] → `goreleaser check` and a snapshot build with `--skip=docker` pass. The
  first real tag push is the first image build, so the README asks for a pre-release tag
  first.
- [ghcr.io visibility is manual] → Documented in the `release` skill, and the registry
  publish is gated by `MCP_REGISTRY_PUBLISH`.
- [OIDC registry login has not been exercised] → Documented. The fallback is a local
  `mcp-publisher login github` followed by `publish`.

## Verification notes

- GoReleaser v2.18.2 field names: `pkg/config/config.go`,
  `www/content/customization/package/archives.md`, `publish/changelog.md` and
  `publish/mcp.md` at tag v2.18.2 in https://github.com/goreleaser/goreleaser.
  - `goreleaser check` passed.
  - `release --snapshot --clean --skip=publish` produced
    `edgex-foundry-mcp_<v>_linux_{amd64,arm64}.tar.gz` and `checksums.txt`.
  - The binaries were static and printed the injected version.
- goreleaser-action v7 (= v7.2.3) inputs are `distribution`, `version` and `args`. It needs
  `GITHUB_TOKEN`, `contents: write` and checkout with `fetch-depth: 0`
  (https://github.com/goreleaser/goreleaser-action, `action.yml`).
- Docker actions are `docker/setup-qemu-action@v4`, `setup-buildx-action@v4` and
  `login-action@v4`.
- MCP registry, from https://github.com/modelcontextprotocol/registry at commit bf4e88c
  (latest tag v1.8.1):
  - schema `https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json`
    (`pkg/model/constants.go`)
  - required fields in `internal/validators/schemas/2025-12-11.json`
  - package types in `docs/modelcontextprotocol-io/package-types.mdx`
  - the OCI label check in `internal/validators/registries/oci.go`
  - namespace and OIDC rules in `docs/modelcontextprotocol-io/authentication.mdx` and
    `github-actions.mdx`
  - case-sensitive name matching in `internal/auth/jwt.go`
  - The proposed `server.json` passed `mcp-publisher validate`. The live registry limits
    descriptions to 100 characters.

## Open Questions

- Signing (cosign) and SBOMs: a follow-up change.
