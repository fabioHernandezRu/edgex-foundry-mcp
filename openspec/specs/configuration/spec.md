# configuration Specification

## Purpose

How edgex-foundry-mcp is configured through flags and environment variables: EdgeX addressing (direct service URLs or secure-mode API gateway), the bearer token, timeouts, result caps, transports, and startup validation.

## Requirements
### Requirement: Configuration sources and precedence
The server SHALL read its configuration from command-line flags and environment variables.
A flag set explicitly on the command line SHALL take precedence over the corresponding
environment variable, and the environment variable SHALL take precedence over the built-in
default.

| Setting | Flag | Env var | Default |
|---|---|---|---|
| core-metadata URL | `--metadata-url` | `EDGEX_METADATA_URL` | `http://localhost:59881` |
| core-data URL | `--data-url` | `EDGEX_DATA_URL` | `http://localhost:59880` |
| core-command URL | `--command-url` | `EDGEX_COMMAND_URL` | `http://localhost:59882` |
| API gateway URL | `--gateway-url` | `EDGEX_GATEWAY_URL` | empty (direct mode) |
| Bearer token (JWT) | none | `EDGEX_TOKEN` | empty |
| Bearer token file | `--token-file` | `EDGEX_TOKEN_FILE` | empty |
| Gateway CA bundle | `--gateway-ca-file` | `EDGEX_GATEWAY_CA_FILE` | empty (system roots) |
| HTTP timeout | `--timeout` | `EDGEX_TIMEOUT` | `10s` |
| Result cap | `--max-results` | `EDGEX_MAX_RESULTS` | `100` |
| Transport | `--transport` | `EDGEX_MCP_TRANSPORT` | `stdio` |
| HTTP listen address | `--http-addr` | `EDGEX_MCP_HTTP_ADDR` | `127.0.0.1:8080` |
| Enable write tools | `--enable-writes` | `EDGEX_ENABLE_WRITES` | `false` |
| Disable live device reads | `--disable-device-reads` | `EDGEX_DISABLE_DEVICE_READS` | `false` |
| Log level | `--log-level` | `EDGEX_LOG_LEVEL` | `info` |

#### Scenario: Defaults target a local non-secure EdgeX
- **WHEN** the server starts with no flags and no `EDGEX_*` environment variables
- **THEN** it uses direct mode with core-metadata at `http://localhost:59881`, core-data at `http://localhost:59880`, core-command at `http://localhost:59882`, stdio transport, a 10s timeout, and write tools disabled

#### Scenario: Flag overrides environment
- **WHEN** `EDGEX_DATA_URL=http://192.0.2.10:59880` is set and the server is started with `--data-url http://192.0.2.20:59880`
- **THEN** the core-data base URL is `http://192.0.2.20:59880`

#### Scenario: Environment overrides default
- **WHEN** `EDGEX_TIMEOUT=3s` is set and no `--timeout` flag is given
- **THEN** the HTTP timeout is 3 seconds

### Requirement: Addressing modes
The server SHALL support two mutually exclusive addressing modes. In direct mode, each
service is reached at its own base URL, and requests go to `<service-url>/api/v3/...`. In
gateway mode (enabled when a gateway URL is set), all services are reached through the
secure-mode API gateway at `<gateway-url>/core-metadata/api/v3/...`,
`<gateway-url>/core-data/api/v3/...` and `<gateway-url>/core-command/api/v3/...`.

#### Scenario: Gateway mode routes by service prefix
- **WHEN** the server is started with `--gateway-url https://192.0.2.10:8443` and a token
- **THEN** a core-data readings request is sent to `https://192.0.2.10:8443/core-data/api/v3/reading/...` and a core-metadata request to `https://192.0.2.10:8443/core-metadata/api/v3/...`

#### Scenario: Gateway mode and explicit service URLs conflict
- **WHEN** the server is started with both `--gateway-url` and an explicit `--data-url` flag
- **THEN** startup fails with a validation error that names both settings, and the server does not start

### Requirement: Bearer token handling
The bearer token SHALL be accepted only from the `EDGEX_TOKEN` environment variable or from
a file named by `--token-file`/`EDGEX_TOKEN_FILE`, never as a flag value. Leading and
trailing whitespace SHALL be trimmed. When a token is configured, it SHALL be sent as
`Authorization: Bearer <token>` on every EdgeX request, in both addressing modes.

#### Scenario: Token read from file
- **WHEN** `--token-file` points to a file containing a JWT followed by a newline
- **THEN** requests carry `Authorization: Bearer <jwt>` without the trailing newline

#### Scenario: Both token sources set
- **WHEN** both `EDGEX_TOKEN` and `--token-file` are set
- **THEN** startup fails with a validation error that does not include the token value

#### Scenario: Gateway mode without a token
- **WHEN** `--gateway-url` is set and no token is configured
- **THEN** the server starts, logs a warning that the gateway will likely answer 401, and does not fail

### Requirement: Configuration validation
The server SHALL validate its configuration at startup and exit with a non-zero status and
a single clear error message when the configuration is invalid. Service and gateway URLs
MUST be absolute `http` or `https` URLs. The timeout MUST be between 1s and 5m.
`--max-results` MUST be between 1 and 1024 (the EdgeX default `MaxResultCount`).
`--transport` MUST be `stdio` or `http`.

#### Scenario: Invalid URL
- **WHEN** the server is started with `--metadata-url localhost:59881`
- **THEN** it exits non-zero with an error stating that the metadata URL must be an absolute http(s) URL

#### Scenario: Result cap above EdgeX maximum
- **WHEN** the server is started with `--max-results 5000`
- **THEN** it exits non-zero with an error stating the allowed range 1..1024

### Requirement: Transport selection
The server SHALL serve MCP over stdio by default. With `--transport http`, it SHALL serve the
MCP streamable HTTP transport at path `/mcp` on `--http-addr`. The default listen address
SHALL be loopback-only.

#### Scenario: Stdio default
- **WHEN** the server starts without `--transport`
- **THEN** it speaks MCP on stdin/stdout, and all logs go to stderr

#### Scenario: HTTP transport on loopback
- **WHEN** the server starts with `--transport http` and no `--http-addr`
- **THEN** it listens on `127.0.0.1:8080` and serves MCP at `/mcp`

### Requirement: Version flag
The server SHALL print its version and exit when started with `--version`.

#### Scenario: Print version
- **WHEN** the binary is run with `--version`
- **THEN** it prints `edgex-foundry-mcp <version>` to stdout and exits 0 without contacting EdgeX

