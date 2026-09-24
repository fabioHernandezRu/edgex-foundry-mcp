## Why

There is no Model Context Protocol server for EdgeX Foundry (LF Edge). As a result, AI
agents cannot answer simple operational questions about an EdgeX deployment, such as
"which devices are down?", "what did this sensor read in the last hour?" or "what commands
does this profile expose?", without someone writing ad-hoc REST calls. A read-only MCP
server over the public EdgeX REST API v3 gives agents safe visibility into a deployment
first. Actuation comes later, behind an explicit opt-in.

## What Changes

- New Go module and binary `edgex-foundry-mcp` (`cmd/edgex-foundry-mcp`), built on the
  official MCP Go SDK, with a stdio transport (default) and a streamable HTTP transport
  (opt-in flag, bound to localhost by default).
- Configuration via flags and environment variables. There are two addressing modes:
  direct per-service base URLs (core-metadata, core-data, core-command), or a single
  secure-mode API gateway URL with a JWT bearer token. Timeouts and result caps are
  configurable.
- A small typed EdgeX HTTP client (`internal/edgex`) for the endpoints the tools need,
  with per-request timeouts, uniform error decoding and no token logging.
- **Read-only** MCP tools (`internal/tools`):
  `system_health`, `list_device_services`, `list_devices`, `get_device`,
  `list_device_profiles`, `get_device_profile`, `get_latest_readings`, `query_readings`,
  `device_data_stats`, `list_device_commands`, `read_device_command`.
- A read-only MCP resource template that exposes device profile documents by name.
- Safety model: read-only by default. No write tool is added in this change, but the
  registration gate (`--enable-writes`) and the redaction of credential-like protocol
  properties are established now, so later write tools plug into an existing, tested
  mechanism.
- Unit tests against `httptest` fake EdgeX services, with payloads modeled on the official
  API specs. No live EdgeX is needed for `go test ./...`.
- Developer experience: `dev/docker-compose.yml` (upstream EdgeX v4, non-secure, with
  device-virtual, pinned tags), a Makefile, GitHub Actions for Go checks next to the
  existing publish-guard and OpenSpec jobs, and a README with quickstart, tool reference
  and safety model.

This change adds **read-only tools only**. Every registered tool maps to HTTP GET
requests that do not change EdgeX metadata or device state. `read_device_command` issues a
core-command GET, which makes the device service read the device. It never sets
`ds-pushevent=true`, so no event is persisted or published as a side effect.

## Non-goals

- Write or actuation tools (core-command SET, adminState/operatingState changes, metadata
  create/update/delete). These are Phase 2, behind `--enable-writes`.
- A verified secure-mode end-to-end example (Phase 2). The gateway/JWT configuration is
  implemented and unit-tested here, but not demonstrated against a live secure stack.
- Support for EdgeX REST API v2 (pre-Minnesota releases) or app-service / rules-engine /
  support-notifications / support-scheduler APIs.
- Release automation, MCP registry publishing, and binaries (Phase 2).
- Authentication of MCP clients connecting over the HTTP transport. It binds to localhost
  by default, and exposing it is the operator's responsibility.

## Capabilities

### New Capabilities

- `configuration`: flags, env vars, addressing modes (direct URLs vs gateway + JWT),
  timeouts, caps, transport selection, and validation errors.
- `edgex-client`: typed HTTP access to core-metadata, core-data and core-command API v3.
  Covers URL building for both addressing modes, auth header, timeouts, pagination
  parameters, and error decoding.
- `mcp-read-tools`: the read-only MCP tools and the device-profile resource template,
  including their inputs, compact output shapes, limits, and error behavior.
- `safety-model`: read-only default, the `--enable-writes` registration gate, hardware
  wording for write tools, redaction of tokens and credential-like protocol properties,
  bounded results, and HTTP binding defaults.
- `dev-tooling`: offline test suite, dev docker-compose stack, Makefile targets, and CI
  checks.

### Modified Capabilities

(none; this is the first change)

## Impact

- New code: `cmd/edgex-foundry-mcp`, `internal/config`, `internal/edgex`,
  `internal/tools`, `scripts/mcpcall` (a small MCP client used for live checks).
- New dependency: `github.com/modelcontextprotocol/go-sdk` (the version is pinned in
  `go.mod`). Everything else uses the standard library.
- New files: `go.mod`, `Makefile`, `.golangci.yml`, `dev/docker-compose.yml`, an extended
  `.github/workflows/ci.yml`, and a rewritten `README.md`.
- External systems: EdgeX core-metadata, core-data and core-command (read-only
  endpoints), reached directly or through the secure-mode API gateway.
