## ADDED Requirements

### Requirement: Write audit log
Every write tool call SHALL emit one log line at level WARN with the tool name, the device,
the command or state field, whether it was a dry run, and the outcome (ok or the error
status). The values written SHALL be logged with credential-like keys redacted. The log
line SHALL never contain the bearer token.

#### Scenario: Audited SET
- **WHEN** `set_device_command` succeeds for `Random-Integer-Device` / `Int8`
- **THEN** the log contains a WARN line with `tool=set_device_command`, the device, the command and `outcome=ok`

### Requirement: No automatic retries for writes
Write tools SHALL NOT retry a failed request. Any failed SET counts toward the device
service's failure tracking and can mark the device DOWN.

#### Scenario: Failed write not retried
- **WHEN** core-command answers 500 to a SET
- **THEN** exactly one PUT was sent, and the tool returns `isError: true`

## MODIFIED Requirements

### Requirement: Write-tool registration gate
The server SHALL keep write/actuation tools in a separate registration set. That set is
registered only when `--enable-writes` (or `EDGEX_ENABLE_WRITES=true`) is set. Every write
tool description MUST state that it can affect physical hardware or modify EdgeX metadata.
Registration SHALL fail if a write tool violates this rule or carries `readOnlyHint: true`.
When `--enable-writes` is set, the server SHALL log a warning at startup that write tools
are enabled, and the server instructions SHALL tell the model to confirm with the user
before calling them.

#### Scenario: Gate closed
- **WHEN** the server starts without `--enable-writes`
- **THEN** no tool from the write set is registered, and the startup log states the server is read-only

#### Scenario: Gate open
- **WHEN** the server starts with `--enable-writes`
- **THEN** the server logs a warning that write tools are enabled, and the tool list equals the write set plus the read-only set

#### Scenario: Write tool descriptions are checked
- **WHEN** a test iterates the write registration set
- **THEN** every write tool's description contains the phrase "physical hardware" or "modifies EdgeX metadata", and has `readOnlyHint: false`
