## Why

Operators want an agent that can do more than observe, for example "lock this device
while I service it" or "set the setpoint to 21". EdgeX exposes actuation through
core-command SET and state changes through core-metadata. Both reach physical hardware,
so they must be available only through the explicit, audited gate that Phase 1
established.

## What Changes

- New **write** tools, registered only with `--enable-writes` (write set in
  `internal/tools`):
  - `set_device_command`: core-command `PUT /api/v3/device/name/{name}/{command}` with a
    `{resourceName: value}` body. The value is validated against the profile's valueType,
    readWrite, and minimum/maximum.
  - `set_device_admin_state`: core-metadata `PATCH /api/v3/device` with `adminState`
    LOCKED/UNLOCKED.
  - `set_device_operating_state`: the same endpoint with `operatingState` UP/DOWN.
- Every description states that the tool affects physical hardware or modifies EdgeX
  metadata. Annotations set `readOnlyHint: false` and `destructiveHint: true`, which the
  existing `validateWriteTool` check enforces.
- An audit log line for every write: tool, device, command/state and result. Values are
  logged only when the resource is not credential-like.
- An optional dry-run parameter that validates without sending.

## Non-goals

- Creating or deleting devices, profiles or device services.
- Bulk writes across many devices.
- Rules-engine or app-service configuration.

## Safety model

This change adds **write tools**. Without `--enable-writes` nothing changes, and the
read-only tests keep asserting that no mutating request is sent. With the flag set, only
the tools above are added, and each goes through the registration validator.

## Capabilities

### New Capabilities
- `mcp-write-tools`: the write tools, input validation and dry-run.

### Modified Capabilities
- `edgex-client`: adds PUT/PATCH methods, which are only reachable from write tools.
- `safety-model`: the registration gate is updated now that write tools exist, and write
  audit logging and no-retry requirements are added.

## Impact

`internal/edgex` (new methods), `internal/tools` (write set), README tool table and safety
section, and new tests with a fake EdgeX that records bodies.
