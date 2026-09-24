## Context

The repository is empty apart from the OpenSpec and Claude Code setup. The target is
EdgeX Foundry v4 (Docker tags `4.0.x`, the latest being `4.0.2`). The author runs it in
production on Raspberry Pi 4 (linux/arm64). This change delivers the first usable server:
read-only visibility into core-metadata, core-data and core-command through MCP, safe for
anyone to point at a production deployment.

Constraints: the official MCP Go SDK, only public EdgeX APIs, a test suite that runs
offline, and the safety model in `openspec/config.yaml`.

## Goals / Non-Goals

**Goals:**
- A small, idiomatic Go codebase: `cmd/` + `internal/config` + `internal/edgex` +
  `internal/tools`.
- 11 read-only tools plus one resource template, with compact, LLM-friendly output.
- A safety gate for future write tools, credential redaction, bounded output, and
  timeouts, all established now and tested.
- Direct and API-gateway addressing, stdio and streamable HTTP transports.
- An offline test suite, a dev stack, CI, and a README good enough for a 5-minute trial.

**Non-Goals:**
- Write/actuation tools (Phase 2), a live secure-mode example (Phase 2), and releases or
  registry publishing (Phase 2).
- Support-* services, app services, rules engine, and EdgeX API v2.
- Authenticating MCP clients on the HTTP transport.

## Decisions

### D1. Go version: minimum 1.25, built and tested with the latest stable (1.27.x)
The SDK v1.8.0 `go.mod` requires `go 1.25.0`. Setting `go 1.25.0` in our `go.mod` lets
users on slightly older distro toolchains build from source, which matters on edge
devices. CI and local builds use the latest stable toolchain (go1.27.1 at the time of
writing), declared in CI via `go-version: stable`.
*Alternative:* `go 1.27` as the minimum. Rejected, because it adds no language feature we
need and excludes users for no benefit.

### D2. MCP SDK pinned at `github.com/modelcontextprotocol/go-sdk v1.8.0`
This is the latest tag. It was verified to build and pass its own tests on go1.27.1.
Typed tools (`mcp.AddTool[In, Out]`) generate input and output schemas from Go structs:
the `jsonschema:"<description>"` tag holds the description, and `omitempty` marks a field
optional. The structured output is also mirrored as a text content item automatically.
*Alternative:* the low-level `Server.AddTool` with hand-written schemas. Rejected because
it adds more code and allows schema drift.

### D3. No dependencies beyond the SDK
The EdgeX client is about 300 lines on `net/http` and `encoding/json`, with only the DTO
fields we use.
*Alternative:* importing `github.com/edgexfoundry/go-mod-core-contracts/v4`. Rejected,
because it pulls in a large dependency tree (and its clients use `limit=-1` style
helpers). We copy only the JSON field names and verify them against the DTO sources listed
below.

### D4. Package boundaries
- `internal/config`: `Config` struct, `Load(args []string, getenv func(string) string)`
  (pure and testable), and `Validate()`. Flags use the standard `flag` package. Precedence
  is flag > env > default, and "explicitly set" is detected with `flag.Visit`.
- `internal/edgex`: `Client` with one `http.Client` (no `DefaultClient`), a
  `baseURL(service)` resolver for direct vs gateway mode, one `get(ctx, svc, path, query,
  out)` helper (timeout via `context.WithTimeout`, bearer header, JSON decode, `*APIError`
  on non-2xx), and typed methods per endpoint. It has no MCP imports.
- `internal/tools`: `Register(server, client, opts)`. It adds the read set always, the
  write set only when `opts.EnableWrites` is set (empty now), and skips
  `read_device_command` when `opts.DisableDeviceReads` is set. It holds the output shaping,
  `redact.go` and `limits.go`.
- `cmd/edgex-foundry-mcp`: loads config, builds a `slog` logger on stderr, the client and
  the server, and picks the transport. It handles signals through
  `signal.NotifyContext`.

### D5. Errors are tool results, not protocol errors
In the SDK, a Go `error` returned from a typed handler becomes a `CallToolResult` with
`isError: true`, which the model can read and act on. Handlers map `*edgex.APIError` to
short, actionable messages: 404 means not found (use `list_*`), 401 means check the token
or use gateway mode, 423 means locked or down, 503 means service busy or timed out.

### D6. Pagination and caps
- Each tool has its own default limit (20 for lists, 10 for latest readings, 50 for
  queries).
- The effective limit is `min(requested, --max-results)`, where `--max-results` is at most
  1024.
- `limit=-1` is never sent. EdgeX rejects `limit > MaxResultCount` (default 1024) with 400,
  so clamping client-side is required, not cosmetic.
- An `offset` past the end gets 416 from EdgeX, which is returned as a tool error.

### D7. Time handling
- Tool inputs take RFC 3339 `start`/`end` or a Go duration `window`.
- The client converts them to Unix nanoseconds, the unit of EdgeX `origin` and of the
  `start`/`end` path parameters.
- Outputs render `origin` (ns) and `created`/`modified` (ms) as RFC 3339 UTC.

### D8. `query_readings` supports one optional resource, not many
EdgeX's multi-resource variant is a GET request with a JSON body
(`{"resourceNames":[...]}`). Proxies, including the gateway, are not guaranteed to forward
a GET body. We support zero or one resource per call, and the model can call the tool
repeatedly.
*Alternative:* fanning out one request per resource on our side. Deferred, because it
complicates pagination.

### D9. `read_device_command` safety
- A core-command GET makes the device service perform a live protocol read. Repeated
  failures update the device's failure tracking and can mark it `DOWN`. With
  `ds-pushevent=true` the event is published to the message bus and persisted by core-data.
- With the default `ds-regexcmd=true`, an unknown command name is treated as a regex over
  resource names.
- Therefore the tool:
  - always sends `ds-pushevent=false&ds-returnevent=true&ds-regexcmd=false`;
  - pre-validates that the command exists with `get: true`;
  - says in its description that it reads the physical device;
  - can be removed with `--disable-device-reads`.
- It keeps `readOnlyHint: true`, because it never actuates or writes EdgeX data. It has
  `openWorldHint` left true, because it touches the physical world.

### D10. Redaction
- A single `redact.Map(map[string]any) map[string]any` deep-copies the map and replaces
  values whose key contains a sensitive substring (case-insensitive):
  `password, passwd, pwd, secret, token, apikey, api_key, key, credential, auth, private,
  cert`.
- It is applied to device `protocols`, device and device-service `properties`, and
  resource `attributes`.
- Over-redaction is accepted.
- Tool errors never include the response body beyond 200 bytes of an EdgeX `message`, and
  never include headers.

### D11. Transports
- **stdio:** `server.Run(ctx, &mcp.StdioTransport{})`.
- **HTTP:** `mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s },
  &mcp.StreamableHTTPOptions{Stateless: true})` mounted at `/mcp`, with
  `http.Server{ReadHeaderTimeout: 10s}`.
  - The server is stateless because it holds no per-session state.
  - The SDK's DNS-rebinding protection is on by default and must not be disabled.
  - The default bind is `127.0.0.1:8080`, and a non-loopback bind logs a warning.

### D12. Server instructions
`ServerOptions.Instructions` gives the model a short orientation:
- Start with `system_health`.
- Discover with `list_devices` and `get_device_profile`.
- Readings are newest-first.
- Everything is read-only unless the server says write tools are enabled.

### D13. Logging
- `log/slog` text handler on stderr, with the level from `--log-level`.
- Logs contain the method, path template, status and latency. They never contain headers,
  tokens or response bodies.
- The SDK logger is set to the same logger. The SDK does not log tool arguments (verified
  in its source), and `mcp.LoggingTransport` is not used.

## Tool list

| Tool | EdgeX endpoint(s) | Service |
|---|---|---|
| `system_health` | `GET /api/v3/ping`, `GET /api/v3/version` (×3 services) | metadata, data, command |
| `list_device_services` | `GET /api/v3/deviceservice/all` | core-metadata |
| `list_devices` | `GET /api/v3/device/all`, `/device/service/name/{name}`, `/device/profile/name/{name}` | core-metadata |
| `get_device` | `GET /api/v3/device/name/{name}` | core-metadata |
| `list_device_profiles` | `GET /api/v3/deviceprofile/basicinfo/all`, `/deviceprofile/manufacturer/{m}`, `/model/{m}`, `/manufacturer/{m}/model/{m}` | core-metadata |
| `get_device_profile` | `GET /api/v3/deviceprofile/name/{name}` | core-metadata |
| `get_latest_readings` | `GET /api/v3/reading/device/name/{name}[/resourceName/{r}]` | core-data |
| `query_readings` | `GET /api/v3/reading/device/name/{name}[/resourceName/{r}]/start/{ns}/end/{ns}` | core-data |
| `device_data_stats` | `GET /api/v3/event/count/device/name/{name}`, `GET /api/v3/reading/count/device/name/{name}`, latest reading (limit 1) | core-data |
| `list_device_commands` | `GET /api/v3/device/name/{name}`, `GET /api/v3/device/all` | core-command |
| `read_device_command` | `GET /api/v3/device/name/{name}/{command}?ds-pushevent=false&ds-returnevent=true&ds-regexcmd=false` | core-command |
| resource `edgex://deviceprofile/{name}` | `GET /api/v3/deviceprofile/name/{name}` | core-metadata |

## Verification notes

All EdgeX facts were checked against the source at the tags below. The scouts cloned the
repositories locally. docs.edgexfoundry.org was not reachable from the build environment,
so its markdown sources in `edgex-docs` were used instead. Nothing was checked against a
running EdgeX yet (see Open Questions).

**Versions**
- `edgex-go` tag `v4.0.2` (same commit as the `palau` branch head).
- `go-mod-core-contracts/v4` tag `v4.0.3` (pinned by edgex-go v4.0.2).
- `go-mod-bootstrap/v4` tag `v4.0.5`.
- `device-sdk-go` tag `v4.0.2`.
- `device-virtual-go` tag `v4.0.2`.
- `edgex-compose` tag `v4.0.2`.
- `edgex-docs` branch `odessa`.

**EdgeX v4 serves REST API v3**
- `common/constants.go` in go-mod-core-contracts v4.0.3 has `ApiVersion = "v3"` and
  `ApiBase = "/api/v3"`.
- Source: https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/common/constants.go

**Ping and version**
- `GET /api/v3/ping` returns `{apiVersion, timestamp, serviceName}`. The timestamp uses Go
  `UnixDate`, so it is not parsed strictly.
- `GET /api/v3/version` returns `{apiVersion, version, serviceName}`. Services built on the
  SDK add `sdk_version`.
- Ping is unauthenticated on direct calls. Through the gateway it still needs a JWT.
- Sources:
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/common/ping.go
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/common/version.go
  - https://github.com/edgexfoundry/go-mod-bootstrap/blob/v4.0.5/bootstrap/controller/commonapi.go
- `GET /api/v3/config` exists but can expose `Writable.InsecureSecrets`. It is
  deliberately not used.

**core-metadata** (port 59881)
- Endpoints:
  - `deviceservice/all`
  - `device/all` (`offset`, `limit`, `labels`)
  - `device/service/name/{name}` and `device/profile/name/{name}`. These take `offset` and
    `limit`, with no labels, and return an empty 200 for unknown names.
  - `device/name/{name}` (404 when not found)
  - `deviceprofile/basicinfo/all`
  - `deviceprofile/manufacturer/{m}`, `deviceprofile/model/{m}`,
    `deviceprofile/manufacturer/{m}/model/{m}`
  - `deviceprofile/name/{name}`
- Response envelopes: `MultiDevicesResponse{totalCount, devices}`,
  `DeviceResponse{device}`, `MultiDeviceProfilesResponse{totalCount, profiles}`,
  `MultiDeviceProfileBasicInfoResponse`, `DeviceProfileResponse{profile}`,
  `MultiDeviceServicesResponse{totalCount, services}`.
- Device DTO fields: `name`, `parent`, `description`, `adminState`, `operatingState`,
  `labels`, `location`, `serviceName`, `profileName`, `autoEvents`, `protocols`
  (map[string]map[string]any), `tags`, `properties`, `created`/`modified` (ms).
- Label filtering is AND (JSONB containment). Postgres returns rows by `created`
  ascending, even though the OpenAPI summary says otherwise.
- Sources:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/openapi/core-metadata.yaml
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/metadata/router.go
  - https://github.com/edgexfoundry/edgex-go/tree/v4.0.2/internal/pkg/infrastructure/postgres
  - https://github.com/edgexfoundry/go-mod-core-contracts/tree/v4.0.3/dtos (`device.go`,
    `deviceservice.go`, `deviceprofile.go`, `deviceprofilebasicinfo.go`, `deviceresource.go`,
    `resourceproperties.go`, `devicecommand.go`, `autoevent.go`)

**core-data** (port 59880)
- Reading endpoints:
  - `reading/device/name/{name}`
  - `.../resourceName/{resourceName}`
  - `.../start/{start}/end/{end}`
  - `.../resourceName/{r}/start/{s}/end/{e}`
- `start` and `end` are Unix **nanoseconds**, matched inclusively against `origin`.
- Results are ordered newest first (`ORDER BY origin DESC`).
- An unknown device returns 200 with an empty list.
- Count endpoints: `event/count/device/name/{name}` and `reading/count/device/name/{name}`
  return `{"count": n}`, with a lowercase field name.
- Reading DTO: `origin`, `deviceName`, `resourceName`, `profileName`, `valueType`,
  `units`, `tags`, and one of `value` (string), `binaryValue` (base64) + `mediaType`, or
  `objectValue`.
- There is no `reading/multiple/...` route. Multiple resources are requested with a GET
  body (see D8).
- Sources:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/openapi/core-data.yaml
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/data/router.go
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/data/controller/http/reading.go
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/reading.go
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/common/count.go

**Pagination rules**
- `offset` defaults to 0. `limit` defaults to 20, and `-1` means `MaxResultCount`.
- `MaxResultCount` defaults to 1024 (`all-services.Service`), and a larger `limit` gets
  400.
- An `offset` greater than the total gets 416.
- Sources:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/pkg/utils/http.go
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/cmd/core-common-config-bootstrapper/res/configuration.yaml

**core-command** (port 59882)
- `device/all` and `device/name/{name}` return `DeviceCoreCommand{deviceName, profileName,
  coreCommands[{name, get, set, path, url, parameters[{resourceName, valueType}]}]}`.
- `GET device/name/{name}/{command}`:
  - Query parameters: `ds-pushevent` (default false), `ds-returnevent` (default true),
    `ds-regexcmd` (default true, handled by the device SDK).
  - Returns `EventResponse{event}`.
  - Status codes: 423 when locked or down, 404 when unknown.
- `PUT device/name/{name}/{command}` is the SET path. It is not used in this change.
- Sources:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/openapi/core-command.yaml
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/command/controller/http/command.go
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/corecommand.go
  - https://github.com/edgexfoundry/device-sdk-go/blob/v4.0.2/internal/controller/http/command.go

**Errors**
- Error bodies are a `BaseResponse{apiVersion, requestId, message, statusCode}`.
- A request timeout is a 503 with a plain-text body `request timeout`, produced by
  `http.TimeoutHandler` from `Service.RequestTimeout`, which defaults to 5s.
- Sources:
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/common/base.go
  - https://github.com/edgexfoundry/go-mod-bootstrap/blob/v4.0.5/bootstrap/handlers/httpserver.go

**Secure mode**
- The gateway is nginx on port 8443, TLS 1.3 only, with a self-signed certificate by
  default.
- It routes `/core-data`, `/core-metadata` and `/core-command` and strips the prefix.
  Every route has `auth_request`, and requests use `Authorization: Bearer <JWT>`.
- A JWT is obtained in three steps:
  1. `secrets-config proxy adduser`
  2. an OpenBao userpass login
  3. `identity/oidc/token/<user>`
- The default TTL is 1h.
- Sources:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/cmd/security-proxy-setup/entrypoint.sh
  - https://github.com/edgexfoundry/edgex-docs/blob/odessa/docs_src/security/Ch-APIGateway.md
  - https://github.com/edgexfoundry/edgex-docs/blob/odessa/docs_src/security/Ch-Authenticating.md
  - https://github.com/edgexfoundry/edgex-compose/blob/v4.0.2/compose-builder/get-api-gateway-token.sh

**Dev stack**
- The non-secure v4 stack is core-keeper (registry and config, no Consul),
  core-common-config-bootstrapper (runs once), core-metadata, core-data, core-command,
  PostgreSQL `postgres:16.3-alpine`, and MQTT `eclipse-mosquitto:2.0.22`.
- All EdgeX images use tag `4.0.2` and are multi-arch (linux/amd64 and linux/arm64/v8).
- Upstream publishes host ports on `127.0.0.1`.
- Sources:
  - https://github.com/edgexfoundry/edgex-compose/blob/v4.0.2/docker-compose-no-secty.yml
  - https://github.com/edgexfoundry/edgex-compose/blob/v4.0.2/compose-builder/.env
  - Docker Hub tag metadata for `edgexfoundry/*:4.0.2`

**device-virtual defaults**
- Devices, each with a profile of the same name and the label `device-virtual-example`:
  - `Random-Boolean-Device` (Bool every 10s)
  - `Random-Integer-Device` (Int8/16/32/64 every 15s)
  - `Random-UnsignedInteger-Device` (every 20s)
  - `Random-Float-Device` (Float32/64 every 30s)
  - `Random-Binary-Device` and `Fixed-Object-Device` (no auto-events)
- Sources:
  - https://github.com/edgexfoundry/device-virtual-go/blob/v4.0.2/cmd/res/devices/devices.yaml
  - https://github.com/edgexfoundry/device-virtual-go/tree/v4.0.2/cmd/res/profiles

**MCP Go SDK v1.8.0**
- Server and tools:
  - `mcp.NewServer(*Implementation, *ServerOptions)`, with `ServerOptions.Instructions`
    and `ServerOptions.Logger *slog.Logger`
  - `mcp.AddTool[In, Out](s, *Tool, ToolHandlerFor[In, Out])`
  - `ToolHandlerFor` is `func(ctx, *CallToolRequest, In) (*CallToolResult, Out, error)`
  - `mcp.ToolAnnotations{ReadOnlyHint bool, IdempotentHint bool, DestructiveHint *bool,
    OpenWorldHint *bool, Title}`
- Resources:
  - `Server.AddResourceTemplate(*ResourceTemplate, ResourceHandler)`. Template variables
    are not passed to the handler, so the name is parsed from `req.Params.URI`.
  - `mcp.ResourceNotFoundError(uri)`
- Transports:
  - `&mcp.StdioTransport{}` with `Server.Run(ctx, t)`
  - `mcp.NewStreamableHTTPHandler(getServer, *StreamableHTTPOptions{Stateless: true})`.
    Localhost protection is on by default.
- Tests: `mcp.NewInMemoryTransports()`, `mcp.NewClient`, `Client.Connect`,
  `ClientSession.CallTool` / `ListTools` / `ReadResource`.
- The minimum Go version for the SDK is 1.25.0.
- A probe program using exactly these APIs was compiled and run on go1.27.1.
- Sources:
  - https://proxy.golang.org/github.com/modelcontextprotocol/go-sdk/@v/v1.8.0.mod
  - https://github.com/modelcontextprotocol/go-sdk/tree/v1.8.0/mcp (`server.go`,
    `protocol.go`, `resource.go`, `streamable.go`, `logging.go`)
  - https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/docs/mcpgodebug.md

## Risks / Trade-offs

- [EdgeX behavior differs from the source-derived contract at runtime, for example the
  exact `version` string or the 401 body] → Tests assert only on documented fields. The
  `edgex-live-check` skill must be run against the dev stack before archiving, and any
  mismatch feeds back into this design.
- [`read_device_command` can mark a flaky device DOWN through repeated failed reads] →
  The description warns the model. The tool performs no automatic retries, and
  `--disable-device-reads` removes it entirely.
- [Redaction by key substring over-redacts (e.g. `AuthMode`) or misses a credential
  stored under an innocuous key] → Over-redaction is accepted. Credentials under
  innocuous keys cannot be detected generically, which is documented in the README.
- [Large profiles or many devices exceed the model context] → Output caps, compact
  shaping, and `nextOffset` pagination.
- [SDK v1.x behavior switches (`MCPGODEBUG`) change defaults in v1.9] → The version is
  pinned. The `mcp-sdk-scout` agent is re-run on any SDK bump.
- [The dev compose file was not run in the authoring environment, which has no Docker
  daemon] → This is stated in the README and the PR. The first live check validates it.

## Migration Plan

This is a new project, so there is nothing to migrate. Rollback means not using the
binary.

## Open Questions

- Runtime confirmation against a live v4.0.2 stack is still pending: the `version`
  string, the 401 body in secure mode, and the ordering of device lists.
- Whether to add `list_events` (`GET /api/v3/event/device/name/{name}`) in a later change.
  It is not needed while readings cover the same data.
