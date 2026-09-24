## ADDED Requirements

### Requirement: Offline test suite
`go test ./...` SHALL pass without a running EdgeX, without Docker, and without network
access. Tests SHALL use `net/http/httptest` fake EdgeX services that serve payloads modeled
on the official EdgeX v4 API specs and DTOs. MCP-level tests SHALL exercise tools through
the SDK's in-memory transports and a real MCP client session.

#### Scenario: Tests run offline
- **WHEN** `go test ./...` runs on a machine with no EdgeX and no outbound network
- **THEN** all tests pass

#### Scenario: Tools tested through the protocol
- **WHEN** the tools test suite runs
- **THEN** each read-only tool is called at least once via an MCP client session connected over in-memory transports

### Requirement: Makefile targets
The repository SHALL provide a Makefile with at least these targets:
- `build`: build `bin/edgex-foundry-mcp` with the version injected
- `test`: `go test -race ./...`
- `lint`: gofmt check, `go vet` and golangci-lint
- `run`: run the server over stdio against the default local URLs
- `dev-up` / `dev-down`: start and stop the dev EdgeX stack

#### Scenario: Build target
- **WHEN** `make build` runs
- **THEN** `bin/edgex-foundry-mcp` exists, and `bin/edgex-foundry-mcp --version` prints a version

### Requirement: Dev EdgeX stack
The repository SHALL provide `dev/docker-compose.yml`. It runs upstream EdgeX Foundry v4 in
non-secure mode with core-keeper, core-common-config-bootstrapper, core-metadata, core-data,
core-command, device-virtual, PostgreSQL and an MQTT broker. All image tags SHALL be pinned
to exact versions, and host ports SHALL be published on `127.0.0.1` only. The images used
SHALL be available for linux/amd64 and linux/arm64.

#### Scenario: Pinned images
- **WHEN** the compose file is inspected
- **THEN** no image uses the `latest` tag or an unpinned tag, and every EdgeX image uses the same exact version

#### Scenario: Loopback-only ports
- **WHEN** the compose file is inspected
- **THEN** every published port mapping begins with `127.0.0.1:`

### Requirement: Continuous integration
GitHub Actions SHALL run on pull requests and on pushes to `main`. The jobs are:
`go vet`, golangci-lint, `go test -race ./...`, `go build ./...`, strict OpenSpec
validation (`scripts/openspec_validate.sh`), and the generic publish-guard scan. Workflow
permissions SHALL be read-only. Strict validation SHALL cover every main spec and every
change that has spec deltas. Roadmap stubs that contain only `proposal.md` SHALL be
listed as skipped, not silently ignored.

#### Scenario: Roadmap stub does not break validation
- **WHEN** `openspec/changes/` contains a change with only `proposal.md`
- **THEN** the validation script reports it as a skipped roadmap stub and still exits 0 when everything else is valid

#### Scenario: CI checks on a pull request
- **WHEN** a pull request is opened
- **THEN** all listed jobs run, and a failure in any of them fails the workflow

### Requirement: Helper MCP client for live checks
The repository SHALL provide `scripts/mcpcall`, a small Go program that starts the server
binary over stdio (or connects to an HTTP endpoint), and then lists tools or calls one tool
with JSON arguments, printing the result. It is used by the `edgex-live-check` skill and is
not part of the server binary.

#### Scenario: List tools through the helper
- **WHEN** `go run ./scripts/mcpcall -- tools/list` is run with the server built
- **THEN** it prints the names of the registered tools

### Requirement: README
`README.md` SHALL state that the project is for EdgeX Foundry (LF Edge) and is unrelated to
any similarly named crypto-exchange project. It SHALL include a 5-minute quickstart (dev
stack, then Claude Desktop / Claude Code configuration, then example prompts), a tool
reference table (tool, read/write, EdgeX endpoint, purpose), the safety model, the
configuration reference, how the project uses OpenSpec and Claude Code (links to
`openspec/specs`, `CLAUDE.md`, `.claude/skills`, `.claude/agents`), and the roadmap.

#### Scenario: Tool table matches registered tools
- **WHEN** a test lists the tools registered in default mode
- **THEN** every tool name appears in the README tool reference table
