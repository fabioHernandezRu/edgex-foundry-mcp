# edgex-foundry-mcp

A [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server that lets AI agents
inspect an **[EdgeX Foundry](https://www.edgexfoundry.org/) (LF Edge)** IoT deployment
through its public REST API v3. It is safe by default, with read-only access, redacted
credentials and bounded output. It is written in Go on the official
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

> **Not a crypto project.** This server is for EdgeX Foundry, the open-source IoT edge
> platform hosted by LF Edge. It is unrelated to any cryptocurrency exchange or project
> that uses the name "edgex".

- **Targets:** EdgeX Foundry v4 (tested against the 4.0.x API contract, REST API `/api/v3`),
  including linux/arm64 deployments such as a Raspberry Pi 4.
- **Transports:** stdio (default) and streamable HTTP.
- **Status:** Phase 1 has read-only tools. Write tools are planned behind an explicit
  opt-in (see [Roadmap](#roadmap)).

## Quickstart (about 5 minutes)

**1. Start an EdgeX stack** (upstream images, non-secure mode, with the virtual device
service). Skip this step if you already run EdgeX.

```sh
make dev-up        # docker compose -f dev/docker-compose.yml up -d
curl -s http://localhost:59881/api/v3/version   # {"apiVersion":"v3","version":"4.0.2",...}
```

**2. Build the server** (Go 1.25 or newer):

```sh
make build         # -> bin/edgex-foundry-mcp
go run ./scripts/mcpcall -- tools/list
go run ./scripts/mcpcall -- call get_latest_readings '{"device":"Random-Integer-Device","limit":3}'
```

**3. Connect your MCP client.**

Claude Code:

```sh
claude mcp add --transport stdio edgex -- /absolute/path/to/bin/edgex-foundry-mcp
# remote EdgeX, direct mode:
claude mcp add --transport stdio --env EDGEX_METADATA_URL=http://192.0.2.10:59881 \
  --env EDGEX_DATA_URL=http://192.0.2.10:59880 --env EDGEX_COMMAND_URL=http://192.0.2.10:59882 \
  edgex -- /absolute/path/to/bin/edgex-foundry-mcp
```

Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "edgex": {
      "command": "/absolute/path/to/bin/edgex-foundry-mcp",
      "env": {
        "EDGEX_METADATA_URL": "http://localhost:59881",
        "EDGEX_DATA_URL": "http://localhost:59880",
        "EDGEX_COMMAND_URL": "http://localhost:59882"
      }
    }
  }
}
```

**4. Ask questions**, for example:

- "Is my EdgeX deployment healthy, and which version is it running?"
- "Which devices are DOWN or LOCKED?"
- "Show the last 5 Int8 readings of Random-Integer-Device and describe the trend."
- "What resources and commands does the Random-Float-Device profile define?"
- "Has Random-Float-Device reported anything in the last hour?"

## Tools

All Phase 1 tools are **read-only**: they only issue HTTP GET requests, and they never
change EdgeX metadata, events or readings.

| Tool | Access | EdgeX endpoint(s) | Purpose |
|---|---|---|---|
| `system_health` | read | `GET /api/v3/ping`, `GET /api/v3/version` on core-metadata, core-data, core-command | Reachability, versions and latency of the core services |
| `list_device_services` | read | core-metadata `GET /api/v3/deviceservice/all` | Device services (protocol adapters) with admin state |
| `list_devices` | read | core-metadata `GET /api/v3/device/all`, `/device/service/name/{name}`, `/device/profile/name/{name}` | Devices with profile, service and admin/operating state; one filter at a time |
| `get_device` | read | core-metadata `GET /api/v3/device/name/{name}` | One device with auto events and **redacted** protocol properties |
| `list_device_profiles` | read | core-metadata `GET /api/v3/deviceprofile/basicinfo/all`, `/deviceprofile/manufacturer/{m}[/model/{m}]`, `/deviceprofile/model/{m}` | Profile summaries |
| `get_device_profile` | read | core-metadata `GET /api/v3/deviceprofile/name/{name}` | Resources (type, R/W, units, range) and commands |
| `get_latest_readings` | read | core-data `GET /api/v3/reading/device/name/{name}[/resourceName/{r}]` | Most recent stored readings, newest first |
| `query_readings` | read | core-data `GET /api/v3/reading/device/name/{name}[/resourceName/{r}]/start/{ns}/end/{ns}` | Readings in a time range (`window` or RFC 3339 `start`/`end`), paginated |
| `device_data_stats` | read | core-data `GET /api/v3/event/count/device/name/{name}`, `/reading/count/device/name/{name}` | Event and reading counts, time of the newest reading |
| `list_device_commands` | read | core-command `GET /api/v3/device/name/{name}`, `/device/all` | Commands with GET/SET support and parameters |
| `read_device_command` | read (live) | core-command `GET /api/v3/device/name/{name}/{command}?ds-pushevent=false&ds-returnevent=true&ds-regexcmd=false` | **Live read of the physical device**; see the safety model |

| Resource | EdgeX endpoint | Purpose |
|---|---|---|
| `edgex://deviceprofile/{name}` | core-metadata `GET /api/v3/deviceprofile/name/{name}` | Device profile document (JSON), including hidden resources |

Output is compact JSON:
- Times are RFC 3339 UTC. EdgeX nanosecond `origin` values are converted.
- Items are referenced by name.
- Lists carry `totalCount`, `count` and `nextOffset`.
- Binary readings are summarized (media type and size), never inlined.

## Safety model

This server can reach IoT hardware, so safety is a feature, not an afterthought.

- **Read-only by default.** Without `--enable-writes`, only tools that issue
  non-mutating GET requests are registered, and each carries the MCP `readOnlyHint`
  annotation. A test calls every tool and asserts that the fake EdgeX never sees a
  PUT/POST/PATCH/DELETE request.
- **Write gate.** Write and actuation tools (planned for Phase 2) live in a separate set,
  registered only with `--enable-writes`. Registration fails unless each description says
  it affects *physical hardware* or *modifies EdgeX metadata*. Enabling writes logs a
  warning.
- **Live device reads are explicit.** `read_device_command` makes the device service read
  the physical device.
  - It always sends `ds-pushevent=false`, so no event is published or stored.
  - It always sends `ds-regexcmd=false`, so the command name is never a regex.
  - It checks first that the command supports GET, and it never retries.
  - Repeated failed reads can make EdgeX mark a device `DOWN`, which its description tells
    the model.
  - Remove it entirely with `--disable-device-reads`.
- **Credential redaction.** EdgeX protocol properties often carry credentials.
  - Values are redacted when their key contains `password`, `passwd`, `pwd`, `secret`,
    `token`, `apikey`, `api_key`, `key`, `credential`, `auth`, `private` or `cert`
    (case-insensitive).
  - This applies to device protocols and properties, device-service properties, and
    resource attributes, and it is recursive.
  - Over-redaction is accepted. A credential stored under an innocuous key name cannot be
    detected.
- **No secrets in logs.**
  - The bearer token is never logged, printed or included in errors.
  - It can only come from `EDGEX_TOKEN` or `--token-file`, never a flag value, so it is
    never visible in the process list.
  - Logs go to stderr only.
- **Bounded everything.**
  - Every EdgeX request has a timeout (`--timeout`, default 10s).
  - Every list is capped by `--max-results` (default 100, maximum 1024, the EdgeX
    `MaxResultCount`), and `limit=-1` is never sent.
  - Long values are truncated.
- **HTTP transport.** It binds to `127.0.0.1:8080` by default and keeps the SDK's
  DNS-rebinding protection. MCP clients are not authenticated, so binding to another
  address logs a warning. Put an authenticating proxy in front if you expose it.

## Configuration

Flags take precedence over environment variables, which take precedence over defaults.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--metadata-url` | `EDGEX_METADATA_URL` | `http://localhost:59881` | core-metadata base URL |
| `--data-url` | `EDGEX_DATA_URL` | `http://localhost:59880` | core-data base URL |
| `--command-url` | `EDGEX_COMMAND_URL` | `http://localhost:59882` | core-command base URL |
| `--gateway-url` | `EDGEX_GATEWAY_URL` | (none) | Secure-mode API gateway, e.g. `https://192.0.2.10:8443`; routes to `/core-*/api/v3/...` |
| n/a | `EDGEX_TOKEN` | (none) | JWT bearer token |
| `--token-file` | `EDGEX_TOKEN_FILE` | (none) | File containing the JWT (alternative to `EDGEX_TOKEN`) |
| `--gateway-ca-file` | `EDGEX_GATEWAY_CA_FILE` | (none) | PEM CA bundle to trust for the gateway (TLS verification is never disabled) |
| `--timeout` | `EDGEX_TIMEOUT` | `10s` | Per-request timeout (1s..5m) |
| `--max-results` | `EDGEX_MAX_RESULTS` | `100` | Cap on items per list (1..1024) |
| `--transport` | `EDGEX_MCP_TRANSPORT` | `stdio` | `stdio` or `http` |
| `--http-addr` | `EDGEX_MCP_HTTP_ADDR` | `127.0.0.1:8080` | Listen address for `--transport http` (endpoint `/mcp`) |
| `--enable-writes` | `EDGEX_ENABLE_WRITES` | `false` | Register write/actuation tools (none exist yet) |
| `--disable-device-reads` | `EDGEX_DISABLE_DEVICE_READS` | `false` | Do not register `read_device_command` |
| `--log-level` | `EDGEX_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `--version` | n/a | n/a | Print the version and exit |

**Secure-mode EdgeX:** use `--gateway-url` together with a JWT. A gateway URL cannot be
combined with per-service URLs. In EdgeX v4 you get a token with
`secrets-config proxy adduser`, then an OpenBao userpass login, then
`identity/oidc/token/<user>`. See the upstream
[API gateway](https://docs.edgexfoundry.org/4.0/security/Ch-APIGateway/) and
[authentication](https://docs.edgexfoundry.org/4.0/security/Ch-Authenticating/) docs. An
end-to-end secure-mode example is on the roadmap.

## Development

```sh
make test     # go test -race ./...  (no EdgeX, Docker or network needed)
make lint     # gofmt, go vet, golangci-lint
make run      # build and run over stdio against localhost
scripts/publish_guard.sh   # secret/confidentiality scan, run before every push
```

Layout:

```
cmd/edgex-foundry-mcp/    main: flags/env, transports, wiring
internal/config/          configuration model and validation
internal/edgex/           typed EdgeX REST API v3 client (+ edgextest fake EdgeX)
internal/tools/           MCP tools, resource, redaction, output shaping
dev/                      docker-compose dev stack (EdgeX 4.0.2 + device-virtual)
scripts/                  mcpcall (MCP client for live checks), publish_guard.sh
```

Tests run tools through a real MCP client session (in-memory transport) against an
`httptest` fake EdgeX. Its payloads are modeled on the upstream OpenAPI specs and the
`go-mod-core-contracts` DTOs.

## How this project is built: OpenSpec and Claude Code

This is a portfolio project, and the engineering process is visible in the repository.

- **Spec-driven development with [OpenSpec](https://github.com/Fission-AI/OpenSpec).**
  Every feature starts as a change proposal in [`openspec/changes/`](openspec/changes).
  A change has a proposal, a design with verification notes that tie every EdgeX
  endpoint to its upstream source, delta specs with testable scenarios, and a task list.
  Completed changes are archived, and their requirements become the living specs in
  [`openspec/specs/`](openspec/specs). The project context and rules are in
  [`openspec/config.yaml`](openspec/config.yaml).
- **[Claude Code](https://code.claude.com) configuration, committed and reusable.**
  - [`CLAUDE.md`](CLAUDE.md): project guide and golden rules (OpenSpec first, verify
    endpoints, safety model, test before done, publish-guard before push).
  - [`.claude/skills/`](.claude/skills):
    - `add-mcp-tool`: end-to-end recipe for one tool
    - `edgex-live-check`: validation against a running EdgeX
    - `scout-feature`: parallel research before a proposal
    - `publish-guard`: pre-push secret and confidentiality scan
    - `release`
    - the OpenSpec workflow skills
  - [`.claude/agents/`](.claude/agents), all read-only:
    - `edgex-api-scout`: upstream EdgeX API contracts
    - `mcp-sdk-scout`: exact MCP Go SDK signatures
    - `safety-reviewer`: diff review against the safety model

## Roadmap

Phase 2 changes are proposed in [`openspec/changes/`](openspec/changes):

- **Write tools behind `--enable-writes`:** core-command SET, and device
  adminState/operatingState.
- **Secure-mode end-to-end example:** API gateway, JWT and TLS.
- **Releases:** goreleaser binaries for linux/amd64 and linux/arm64, and publishing to the
  MCP registry.
- **Demo:** an animated GIF in this README.

## License

[Apache-2.0](LICENSE), the same license as EdgeX Foundry. EdgeX Foundry is a project of LF
Edge. This project is independent and not affiliated with or endorsed by LF Edge or the
Linux Foundation.
