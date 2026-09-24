## ADDED Requirements

### Requirement: Read-only by default
Without `--enable-writes`, the server SHALL register only tools whose EdgeX calls are HTTP
GET requests that do not create, update or delete EdgeX metadata, events or readings, and
that do not actuate devices. Every such tool SHALL carry the MCP annotation
`readOnlyHint: true`.

#### Scenario: Default tool set is read-only
- **WHEN** the server starts without `--enable-writes` and a client lists tools
- **THEN** every listed tool has `readOnlyHint: true`

#### Scenario: Default server never issues mutating HTTP methods
- **WHEN** every registered tool is called against a fake EdgeX in the test suite
- **THEN** the fake EdgeX observes no PUT, POST, PATCH or DELETE request

### Requirement: Write-tool registration gate
The server SHALL keep write/actuation tools in a separate registration set. That set is
registered only when `--enable-writes` (or `EDGEX_ENABLE_WRITES=true`) is set. Every write
tool description MUST state that it can affect physical hardware or modify EdgeX metadata.
This change registers no write tools. The gate exists so that later changes add write tools
only through it. When `--enable-writes` is set, the server SHALL log a warning at startup
that write tools are enabled.

#### Scenario: Gate closed
- **WHEN** the server starts without `--enable-writes`
- **THEN** no tool from the write set is registered, and the startup log states the server is read-only

#### Scenario: Gate open with no write tools yet
- **WHEN** the server starts with `--enable-writes`
- **THEN** the server logs a warning that write tools are enabled, and the tool list equals the write set plus the read-only set (the write set being empty in this change)

#### Scenario: Write tool descriptions are checked
- **WHEN** a test iterates the write registration set
- **THEN** every write tool's description contains the phrase "physical hardware" or "modifies EdgeX metadata", and has `readOnlyHint: false`

### Requirement: Live device reads are explicit and can be disabled
The server SHALL treat `read_device_command` as a live read of the physical device through
its device service. It SHALL be registered by default as read-only (it never actuates and never persists
events). Its description SHALL state that it triggers a live read of the physical device,
and that repeated failures can cause EdgeX to mark the device operating state DOWN. It SHALL
NOT be registered when `--disable-device-reads` is set.

#### Scenario: Device reads disabled
- **WHEN** the server starts with `--disable-device-reads`
- **THEN** `read_device_command` is not in the tool list, and every other read-only tool is

### Requirement: Credential redaction in tool output
Before returning device, device service or device profile data, the server SHALL redact
the values of map entries whose key contains, case-insensitively, any of these substrings: `password`,
`passwd`, `pwd`, `secret`, `token`, `apikey`, `api_key`, `key`, `credential`, `auth`,
`private`, `cert`, `community`. In addition, credentials embedded in URL-like string values
(`scheme://user:password@host`) SHALL be replaced by `scheme://***REDACTED***@host`, both in
redacted maps and in EdgeX error messages returned to the model. This applies to device
`protocols` (every protocol's property map),
device `properties`, `tags` and `location`, device service `properties`, and resource
`attributes`. The key is
kept and the value is replaced with the string `***REDACTED***`. Over-redaction (for
example of a key named `AuthMode`) is accepted in exchange for never leaking a credential. Redaction SHALL be applied
recursively to nested maps.

#### Scenario: Protocol password redacted
- **WHEN** `get_device` returns a device whose protocols are `{"mqtt":{"Host":"192.0.2.10","Password":"example-pass","Username":"reader"}}`
- **THEN** the tool output contains `"Password":"***REDACTED***"`, keeps `"Host":"192.0.2.10"` and `"Username":"reader"`, and does not contain `example-pass` anywhere

#### Scenario: URL credentials scrubbed
- **WHEN** a protocol property is `"Endpoint": "opc.tcp://user:example-pw@192.0.2.10:4840"`, or an EdgeX error message contains such a URL
- **THEN** the returned value is `opc.tcp://***REDACTED***@192.0.2.10:4840`

#### Scenario: Nested secret redacted
- **WHEN** a device property is `{"opcua":{"security":{"PrivateKeyPath":"/keys/a.pem"}}}`
- **THEN** the value of `PrivateKeyPath` is `***REDACTED***`

### Requirement: No secrets in logs or errors
The server SHALL NOT write the bearer token, the `Authorization` header, or unredacted
protocol/property values to logs, to tool error messages, or to tool output. Logs SHALL
go to stderr only.

#### Scenario: Token absent from logs
- **WHEN** the server runs a full test session at debug log level with token `test-token-value`, and one EdgeX call fails with 401
- **THEN** the captured logs and all tool results contain no occurrence of `test-token-value`

### Requirement: Bounded results
Device profile output (tool and resource) SHALL keep at most `--max-results` resources
and `--max-results` commands and flag the output as truncated. It SHALL also truncate
long descriptions, default values and attribute strings. Every list or query tool SHALL
cap the number of returned items at the smaller of the
tool's requested limit and the configured `--max-results`. It SHALL report `totalCount`
(when EdgeX provides it) and a `nextOffset` when more items exist. Binary reading values
SHALL NOT be returned inline. Only their media type and size are returned. Individual
string values longer than 1024 bytes SHALL be truncated and flagged.

#### Scenario: Cap applied
- **WHEN** `list_devices` is called with `limit: 500` and `--max-results 100`
- **THEN** EdgeX is queried with `limit=100`, and the result includes `nextOffset` when `totalCount` exceeds `offset + 100`

#### Scenario: Binary reading
- **WHEN** a reading has `valueType` `Binary` with a 40 KB `binaryValue`
- **THEN** the tool output includes `mediaType` and `binarySize` for that reading, and no base64 payload

### Requirement: Token confined to its destination
The EdgeX client SHALL NOT follow HTTP redirects, so a redirect can never resend the
bearer token to another scheme or host. The server SHALL log a warning when a token is used
with an `http://` gateway URL to a non-loopback host. Names used as path segments SHALL be
rejected when empty, `.` or `..`, so they cannot alter the request route.

#### Scenario: Redirect not followed
- **WHEN** an EdgeX endpoint answers 301 with a `Location` header
- **THEN** the client returns an error with status 301 and makes no request to the redirect target

#### Scenario: Dot segment rejected
- **WHEN** `get_device` is called with name `..`
- **THEN** the tool returns an error and no request reaches EdgeX

### Requirement: Safe HTTP transport defaults
The streamable HTTP transport SHALL bind to a loopback address by default. It SHALL keep
the SDK's DNS-rebinding (localhost) protection enabled, and apply request header, read
and idle timeouts. Binding to a non-loopback address SHALL log a warning that the MCP endpoint has no
client authentication.

#### Scenario: Non-loopback bind warns
- **WHEN** the server starts with `--transport http --http-addr 0.0.0.0:8080`
- **THEN** it logs a warning that the MCP endpoint is unauthenticated and reachable from the network
