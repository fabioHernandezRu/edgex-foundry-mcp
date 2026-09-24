---
name: release
description: Use when the user asks to cut, tag or publish a release of edgex-foundry-mcp, bump its version, generate the changelog, build release binaries or the container image, or publish to the MCP registry. Covers the version source of truth (git tags), the GoReleaser config, the tag-driven release workflow, ghcr.io visibility and the gated MCP registry publish. Never pushes a tag without the user's explicit confirmation.
---

# Release

Everything is driven by **annotated git tags** `vMAJOR.MINOR.PATCH` (SemVer). There is no
VERSION file. A tag push runs `.github/workflows/release.yml`:

1. `make test` and `scripts/publish_guard.sh --ci --files-only` run first.
2. GoReleaser v2.18.2 (`.goreleaser.yaml`) builds and publishes:
   - static binaries for linux/amd64 and linux/arm64 (Raspberry Pi 4 and up), as tar.gz
     archives with LICENSE and README
   - `checksums.txt`
   - a changelog grouped into Features and Bug fixes from conventional commits
   - the multi-arch image `ghcr.io/fabiohernandezru/edgex-foundry-mcp:<version>`, labeled
     for the MCP registry
3. The `mcp-registry` job publishes `server.json`, with its version and image tag
   rewritten from the tag. It runs only when the repository variable
   `MCP_REGISTRY_PUBLISH` is `true`, and never for pre-release tags (`-rc.1` and similar).

## Procedure

1. Run `git pull`. CI must be green on `main`. Every OpenSpec change that belongs to this
   release is archived.
2. Run the `publish-guard` skill.
3. Choose the version from the commits since the last tag (`git log $(git describe --tags --abbrev=0)..HEAD --oneline`):
   `feat` means minor, `fix` means patch, and `!`/`BREAKING CHANGE` means major. Before
   1.0.0, breaking changes bump the minor version.
4. Dry run locally with `make release-snapshot` (no Docker needed). Check `dist/` for both
   archives and `checksums.txt`. `dist/*_linux_amd64_v1/edgex-foundry-mcp --version`
   should print the snapshot version.
5. **Ask the user to confirm** the version, then run
   `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`. For the first release, prefer
   a pre-release tag such as `v0.1.0-rc.1`, because the image build is only exercised in
   Actions.
6. Watch the workflow. Check the GitHub release assets and changelog, and the ghcr.io
   image for both platforms.

## One-time setup (the repository owner does this)

- **ghcr.io visibility:** after the first image push, open the package settings on GitHub
  and set the visibility to **public**. The MCP registry verifies the
  `io.modelcontextprotocol.server.name` label anonymously.
- **Registry publishing:** then set the repository variable `MCP_REGISTRY_PUBLISH=true`
  (Settings > Secrets and variables > Actions > Variables). The job logs in with GitHub
  OIDC, and the server name must match the GitHub login exactly
  (`io.github.fabioHernandezRu/...`, which is case-sensitive).
- Fallback: publish manually with `mcp-publisher login github` and then
  `mcp-publisher publish`, using a `server.json` whose version and tag match the release.

## Rules

- Never push a tag, delete a tag, or re-run a release without the user's explicit
  confirmation.
- Never edit `server.json`'s `name`. It must equal the OCI label in `.goreleaser.yaml`
  (`scripts/release_test.go` enforces this).
