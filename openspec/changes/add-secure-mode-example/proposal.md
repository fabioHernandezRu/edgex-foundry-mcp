> **Status: stub (Phase 2).** Only the proposal exists.

## Why

Production EdgeX runs in secure mode: an nginx API gateway on 8443, JWT authentication
and OpenBao. Phase 1 implements gateway mode and bearer tokens and unit-tests them, but
there is no demonstrated end-to-end path. A verified example removes the biggest adoption
hurdle.

## What Changes

- `dev/docker-compose.secure.yml`, derived from edgex-compose v4.0.x
  `docker-compose.yml`: security-bootstrapper, secret store, proxy setup/auth, nginx
  gateway, core services and device-virtual. Tags are pinned and ports are
  loopback-only.
- `scripts/get-token.sh`, which follows the upstream flow: `secrets-config proxy adduser`,
  userpass login, then `identity/oidc/token`. It writes the JWT to a file with mode 0600
  for `--token-file`, and never prints it.
- Documentation of the self-signed gateway certificate and how to extract it for
  `--gateway-ca-file`. TLS verification stays mandatory.
- Handling of expired tokens: a clear tool error suggesting a token refresh. Automatic
  token refresh is out of scope.
- The `edgex-live-check` skill gains a secure-mode section.

## Non-goals

- Automatic token acquisition or refresh inside the server.
- mTLS or zero-trust (OpenZiti) deployments.

## Safety model

No new tools, so it stays read-only. Token handling keeps the Phase 1 guarantees:
env/file only, never logged.

## Capabilities

### Modified Capabilities
- `configuration`: expired-token error behavior.
- `dev-tooling`: the secure dev stack and token script.

## Impact

`dev/`, `scripts/`, README secure-mode section, and `.claude/skills/edgex-live-check`.
