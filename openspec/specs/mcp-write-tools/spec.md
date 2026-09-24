# mcp-write-tools Specification

## Purpose

The write tools, registered only with --enable-writes, that actuate devices through core-command SET and change device admin/operating state in core-metadata. They validate before sending, support dry runs, and are audited.

## Requirements
### Requirement: Write tools exist only behind the write gate
The server SHALL provide the write tools `set_device_command`, `set_device_admin_state` and
`set_device_operating_state`. It SHALL register them only when `--enable-writes` is set,
through the write registration set. Each write tool SHALL carry `readOnlyHint: false`. Its
description SHALL state that it affects physical hardware or modifies EdgeX metadata.

#### Scenario: Write tools absent by default
- **WHEN** the server starts without `--enable-writes` and a client lists tools
- **THEN** none of `set_device_command`, `set_device_admin_state` or `set_device_operating_state` is listed

#### Scenario: Write tools present with the gate open
- **WHEN** the server starts with `--enable-writes`
- **THEN** the three write tools are listed with `readOnlyHint: false`, together with all read-only tools

### Requirement: set_device_command tool
The `set_device_command` tool (gated by `--enable-writes`) SHALL issue
`PUT /api/v3/device/name/{device}/{command}` on core-command. The body is a JSON object that
maps each resource name to its value, encoded as a string. Inputs are `device`, `command`,
`values` (resource name → string value) and optional `dryRun` (default false).

Before sending anything, the tool SHALL validate:
- the command exists in core-command for the device and has `set: true`
- `values` contains exactly the command's parameters: every parameter is present and no
  unknown key is given
- each value parses for the parameter's valueType (integer bit width, unsigned, float,
  bool). Array types must be JSON array text. Binary types are rejected because EdgeX does
  not support Binary SET over REST.
- numeric scalar values respect the resource's `minimum`/`maximum` in the device profile,
  when those are defined

With `dryRun: true`, the tool SHALL perform all validation, return the request it would
send, and issue no PUT. A successful call SHALL return the device, the command and the
values written.

#### Scenario: Successful SET
- **WHEN** it is called with `device: "Random-Integer-Device"`, `command: "Int8"`, `values: {"Int8": "42"}` and the server has `--enable-writes`
- **THEN** core-command receives `PUT /api/v3/device/name/Random-Integer-Device/Int8` with body `{"Int8":"42"}`, and the tool reports success

#### Scenario: Command not settable
- **WHEN** the command exists but has `set: false`
- **THEN** the tool returns `isError: true`, lists the settable commands, and sends no PUT

#### Scenario: Missing or unknown value
- **WHEN** `values` omits a parameter of the command, or includes a key that is not a parameter
- **THEN** the tool returns `isError: true` naming the expected parameters, and sends no PUT

#### Scenario: Out-of-range value
- **WHEN** `values` is `{"Int8": "200"}` and the resource maximum is 100 (or the value overflows int8)
- **THEN** the tool returns `isError: true` explaining the allowed range, and sends no PUT

#### Scenario: Dry run
- **WHEN** a valid call includes `dryRun: true`
- **THEN** the tool returns the method, path and body it would send, and core-command receives no PUT

#### Scenario: Device locked or down
- **WHEN** core-command answers 423
- **THEN** the tool returns `isError: true` stating that the device or its service is locked or down

### Requirement: Device state tools
The `set_device_admin_state` tool (gated by `--enable-writes`) SHALL set a device's
`adminState` to `LOCKED` or `UNLOCKED`. The `set_device_operating_state` tool (gated by
`--enable-writes`) SHALL set `operatingState` to `UP`, `DOWN` or `UNKNOWN`. Both SHALL use
`PATCH /api/v3/device` on core-metadata, with a single-item JSON array
`[{"apiVersion":"v3","device":{"name":"<device>","<field>":"<state>"}}]`. They SHALL NOT
send `protocols` or any other device field. Both accept an optional `dryRun`. The 207
per-item status SHALL be checked: a non-200 item status is a tool error carrying the
item's message. `set_device_operating_state` SHALL state in its description that a DOWN
device rejects every command with 423 until it is set back to UP.

#### Scenario: Lock a device
- **WHEN** `set_device_admin_state` is called with `device: "Random-Integer-Device"`, `state: "LOCKED"`
- **THEN** core-metadata receives `PATCH /api/v3/device` with body `[{"apiVersion":"v3","device":{"name":"Random-Integer-Device","adminState":"LOCKED"}}]`, and the tool reports success

#### Scenario: Per-item failure in a 207 response
- **WHEN** core-metadata answers 207 with an item `{"statusCode":404,"message":"device not found"}`
- **THEN** the tool returns `isError: true` with that message

#### Scenario: Invalid state
- **WHEN** `set_device_operating_state` is called with `state: "BROKEN"`
- **THEN** the tool returns `isError: true` listing UP, DOWN and UNKNOWN, and sends no request

