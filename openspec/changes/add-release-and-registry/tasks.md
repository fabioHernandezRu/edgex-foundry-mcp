## 1. Build and package

- [ ] 1.1 Add `.goreleaser.yaml` (v2): linux amd64/arm64 static builds, tar.gz with LICENSE and README, checksums, grouped changelog, `dockers_v2` multi-arch image with the registry label
- [ ] 1.2 Add `Dockerfile` (distroless static nonroot) for the GoReleaser image build
- [ ] 1.3 Add `make release-snapshot` (snapshot without publish or docker) and run it; confirm the archives, checksums and the injected version

## 2. Registry metadata

- [ ] 2.1 Add `server.json` (schema 2025-12-11, OCI package, stdio, EDGEX_* env vars, token marked secret)
- [ ] 2.2 Add `scripts/release_test.go` asserting consistency (name equals label, description of at most 100 characters, secret token, schema URL)

## 3. Workflows

- [ ] 3.1 Add `.github/workflows/release.yml`: `v*` tags; test and publish-guard; GoReleaser pinned to v2.18.2 with QEMU/Buildx/ghcr login; a gated `mcp-publisher` OIDC publish job
- [ ] 3.2 Add a `goreleaser check` step to CI

## 4. Docs

- [ ] 4.1 Wire up the `release` skill (procedure, ghcr visibility, registry variable) and update CLAUDE.md
- [ ] 4.2 README: install from releases, container usage, roadmap update

## 5. Verification

- [ ] 5.1 `goreleaser check`, `make release-snapshot`, `make test lint`, `scripts/openspec_validate.sh` and `scripts/publish_guard.sh` are clean
- [ ] 5.2 Record what could not be verified (docker build, Actions run, registry publish)
