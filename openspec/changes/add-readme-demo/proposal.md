> **Status: stub (Phase 2).** Only the proposal exists.

## Why

A short visual demo shows the value of the server in seconds. It shows an agent answering
"which devices are down and what did they last report?" against a real EdgeX, which
matters for a portfolio project.

## What Changes

- A reproducible demo script in `scripts/demo/`: dev stack, the server, and a scripted
  MCP client session or a recorded Claude Code session.
- An animated GIF (or SVG terminal cast) under `docs/`, embedded near the top of the
  README. It is kept small (under 2 MB) and contains only the dev stack's neutral example
  names.

## Non-goals

- Hosting a live demo instance.

## Safety model

No tool changes. The recording must pass publish-guard: no real hostnames, IPs or tokens.

## Capabilities

### Modified Capabilities
- `dev-tooling`: the demo script and asset.

## Impact

`docs/`, `scripts/demo/`, `README.md`.
