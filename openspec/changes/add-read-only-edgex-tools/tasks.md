## 1. Project scaffolding

- [x] 1.1 Create `go.mod` (`module github.com/fabioHernandezRu/edgex-foundry-mcp`, `go 1.25.0`) and add `github.com/modelcontextprotocol/go-sdk v1.8.0`
- [x] 1.2 Add `Makefile` with `build` (version via `-ldflags -X main.version`), `test` (`go test -race ./...`), `lint` (gofmt check, `go vet`, golangci-lint), `run`, `dev-up`, `dev-down`
- [x] 1.3 Add `.golangci.yml` (golangci-lint v2 config: default linters plus `gosec`, `bodyclose`, `noctx`, `errorlint`, `misspell`, `revive`)

## 2. Configuration (`internal/config`)

- [x] 2.1 Implement the `Config` struct and `Load(args, getenv)` with flag > env > default precedence, for every setting in the configuration spec
- [x] 2.2 Implement gateway mode vs direct-URL conflict detection, and token loading from `EDGEX_TOKEN` or `--token-file` (trimmed; both set is an error)
- [x] 2.3 Implement `Validate()`: absolute http(s) URLs, timeout 1s..5m, max-results 1..1024, transport stdio|http; error messages never include the token
- [x] 2.4 Write table-driven tests covering every configuration spec scenario

## 3. EdgeX client (`internal/edgex`)

- [x] 3.1 Implement `Client` construction: dedicated `http.Client`, optional gateway CA bundle, direct/gateway base-URL resolver per service
- [x] 3.2 Implement the shared `get` helper: per-request timeout context, bearer header, path escaping, query encoding, JSON decoding, `*APIError` (status, service, EdgeX message or 200-byte text excerpt) with `IsNotFound`/`IsUnauthorized`/`IsLocked` helpers
- [x] 3.3 Add a limit clamp helper (never `-1`, never above the cap) and unit tests
- [x] 3.4 Add DTOs (only the fields used) for ping, version, device service, device, device profile (basic info + full), reading, event, count, device core command
- [x] 3.5 Implement core-metadata methods: ping, version, device services, devices (all/by service/by profile), device by name, profiles (basic info/by manufacturer/by model/by both), profile by name
- [x] 3.6 Implement core-data methods: ping, version, readings by device (+resource) with and without time range, event count and reading count by device
- [x] 3.7 Implement core-command methods: ping, version, device commands (all/by device), GET command with `ds-pushevent=false&ds-returnevent=true&ds-regexcmd=false`
- [x] 3.8 Add `edgextest` fake EdgeX with `testdata/` fixtures modeled on the official v4.0.2 OpenAPI examples and DTOs (neutral names, RFC 5737 IPs)
- [x] 3.9 Write tests: GET-only, path escaping, pagination/labels query, gateway prefixes, bearer header present/absent, timeout, JSON 404, plain-text 503, 401, safe command flags

## 4. Safety primitives (`internal/tools`)

- [x] 4.1 Implement `redact.go` (recursive, case-insensitive key-substring redaction to `***REDACTED***`, deep copy) with tests for protocols, nested maps and non-matching keys
- [x] 4.2 Implement output shaping helpers: RFC 3339 time conversion (ns/ms), reading formatting (string/number values, binary → `mediaType`+`binarySize`, object values, 1024-byte string truncation), and list envelope (`totalCount`, `offset`, `count`, `nextOffset`)
- [x] 4.3 Implement `Register(server, client, Options)` with separate read and write sets, the `--enable-writes` gate (empty write set plus warning), `--disable-device-reads`, and the EdgeX-error-to-tool-error mapping
- [x] 4.4 Write tests: default tool list is all `readOnlyHint`; the write-set description check; the write gate logs a warning; `--disable-device-reads` removes only `read_device_command`

## 5. Read-only tools (`internal/tools`), each with a test and a README table row

- [x] 5.1 `system_health`: concurrent ping+version for 3 services, partial failure → `healthy:false`; test; README row
- [x] 5.2 `list_device_services`: test; README row
- [x] 5.3 `list_devices` (service | profile | labels, mutually exclusive): test (incl. conflicting filters, empty result hint); README row
- [x] 5.4 `get_device` (redacted protocols/properties): test (incl. 404 → tool error, redaction); README row
- [x] 5.5 `list_device_profiles` (basic info / manufacturer / model): test; README row
- [x] 5.6 `get_device_profile` (resources + commands, `includeHidden`): test; README row
- [x] 5.7 `get_latest_readings`: test (incl. binary and empty results); README row
- [x] 5.8 `query_readings` (`start`/`end` or `window`): test (incl. invalid and ambiguous ranges); README row
- [x] 5.9 `device_data_stats`: test (incl. device with no data); README row
- [x] 5.10 `list_device_commands`: test; README row
- [x] 5.11 `read_device_command` (pre-validation, safe flags, 423 mapping): test; README row
- [x] 5.12 Resource template `edgex://deviceprofile/{name}`: test (incl. percent-decoding and not-found); README row
- [x] 5.13 Protocol-level test: all tools via in-memory transports; the fake EdgeX sees no mutating method; the token never appears in logs or results

## 6. Server binary (`cmd/edgex-foundry-mcp`)

- [x] 6.1 Implement `main`: `--version`, config load/validate, stderr `slog` logger, client, server with `Instructions`, tool registration
- [x] 6.2 Implement the stdio transport and the streamable HTTP transport (`/mcp`, stateless, `ReadHeaderTimeout`, warning on non-loopback bind), with graceful shutdown on SIGINT/SIGTERM
- [x] 6.3 Write tests for the HTTP bind warning and the `--version` output

## 7. Dev tooling

- [x] 7.1 Add `dev/docker-compose.yml` (EdgeX 4.0.2 non-secure: core-keeper, common-config-bootstrapper, core-metadata, core-data, core-command, device-virtual, postgres 16.3-alpine, mosquitto 2.0.22; loopback ports; pinned tags); validated with `docker compose config` (no Docker daemon in the authoring environment: the stack itself was not started)
- [x] 7.2 Add `scripts/mcpcall` (Go MCP client over stdio/HTTP: `tools/list`, `call <tool> <json>`)
- [x] 7.3 Add a test that asserts compose tags are pinned and ports are loopback-only

## 8. CI

- [x] 8.1 Extend `.github/workflows/ci.yml` with a Go job: `go vet`, golangci-lint, `go test -race ./...`, `go build ./...` (latest stable Go), alongside the existing publish-guard and openspec jobs

## 9. Documentation

- [x] 9.1 Rewrite `README.md`: EdgeX Foundry (LF Edge) disambiguation, quickstart (dev stack → Claude Desktop / Claude Code config → example prompts), tool table, safety model, configuration reference, OpenSpec + Claude Code workflow links, roadmap
- [x] 9.2 Add a test that every registered tool appears in the README tool table
- [ ] 9.3 Update `CLAUDE.md` and the skills if commands or paths changed during implementation

## 10. Verification

- [ ] 10.1 Run `make test lint`, `openspec validate --all --strict` and `scripts/publish_guard.sh`; all must be clean
- [ ] 10.2 Run the `safety-reviewer` agent on the full diff and address its findings
- [ ] 10.3 Run the `edgex-live-check` skill if a live EdgeX is available; otherwise record that it was not run
