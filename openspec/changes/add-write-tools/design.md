## Context

Phase 1 shipped read-only tools and an empty write set behind `--enable-writes`, with a
registration validator. This change adds the first write tools: actuation through
core-command SET, and device admin/operating state through core-metadata. Both reach
physical hardware or change how EdgeX treats it, so the design is conservative. Everything
is validated before sending, there is a dry run and an audit log, and nothing is retried.

## Goals / Non-Goals

**Goals:**
- `set_device_command`, `set_device_admin_state` and `set_device_operating_state`,
  registered only with `--enable-writes`.
- Client-side validation that catches what EdgeX would reject, with better messages,
  before any request is sent.
- Every write is auditable (a WARN log line), and every write can be dry-run.

**Non-Goals:**
- Creating or deleting devices, profiles or services, and bulk writes.
- Binary SET (EdgeX does not support it over REST).
- Changing a device service's own adminState (`PATCH /api/v3/deviceservice`).
- Per-device allowlists (they could come later as configuration).

## Decisions

### D1. Values are sent as strings
- The SET body is `{"<resource>": "<value>"}` with every value a JSON string.
- The device SDK applies `fmt.Sprint` to each value before parsing it. A native JSON array
  becomes `"[1 2 3]"` and fails, and a large int64 sent as a JSON number turns into
  `1e+21`.
- Strings are the one encoding that works for every supported scalar and array type. They
  also match the OpenAPI `SettingRequest` schema.
- *Alternative:* typed JSON values. Rejected, for the reasons above.
- Object and ObjectArray values are sent as JSON text strings, which the SDK accepts
  through `normalizeToObject`.

### D2. Exactly the command's parameters, validated before sending
The command's parameter list comes from core-command `GET /device/name/{name}` (as in
Phase 1). The tool then requires:
- `set: true`, which avoids a 405 from EdgeX;
- every parameter present and no unknown keys. EdgeX silently ignores extra keys and falls
  back to defaults for missing ones (or returns 500), which hides mistakes;
- each value parses for its valueType:
  - `IntN` uses `ParseInt(bits)`, `UintN` uses `ParseUint(bits)`, and `Float32/64` use
    `ParseFloat`;
  - `Bool` uses `ParseBool`, and `String` accepts anything;
  - `*Array` values must be valid JSON array text;
  - `Object` values must be valid JSON;
  - `Binary` is rejected;
- `minimum`/`maximum` from the device profile are respected for numeric scalars. The
  profile comes from core-metadata `GET /device/name/{name}`, then
  `GET /deviceprofile/name/{profile}`. EdgeX enforces these only when
  `Device.DataTransform` is true, but checking them always is safer.

Empty strings are rejected for all non-String types, which mirrors the SDK.

### D3. State changes via the batch PATCH, one item, minimal fields
- The body is `[{"apiVersion":"v3","device":{"name":"<device>","adminState":"<state>"}}]`.
- `protocols` and all other fields are left out, so nothing is overwritten and no
  credentials are ever sent.
- `bypassValidation` stays at its default (false), so metadata asks the owning device
  service to validate the change, as upstream intends.
- The response is 207 with one `BaseResponse` per item. A non-200 item status is turned
  into a tool error with the item's message.
- A top-level 400 is returned when DTO validation fails.

### D4. Client methods
- `edgex.Client.SetCommand(ctx, device, command, values map[string]string) error`: PUT.
- `edgex.Client.UpdateDeviceState(ctx, device, adminState, operatingState string) error`:
  PATCH. It sets exactly one of the two fields.
- Both go through the shared request path, which gains a method and an optional JSON body
  with `Content-Type: application/json`. Timeouts, the no-redirect rule, name validation
  and error decoding are the same as for reads.
- Read methods keep using GET only, and the Phase 1 test that asserts "only GET" becomes
  "only GET from read methods".

### D5. Annotations
| Tool | readOnlyHint | destructiveHint | idempotentHint |
|---|---|---|---|
| `set_device_command` | false | true | false |
| `set_device_admin_state` | false | false | true |
| `set_device_operating_state` | false | true | true |

Setting a device to DOWN makes every command fail with 423 until it is set back to UP, so
that tool is marked destructive and says so in its description.

### D6. Audit log and no retries
- Each write logs one WARN line `edgex write` with `tool`, `device`, `command` or `field`,
  `dryRun`, the redacted `values` or `state`, and `outcome` (ok or the error).
- Writes are never retried. Any failed SET, including a 400 or 423, counts as a failure in
  the device service. With `AllowedFails > 0` it can mark the device DOWN.

### D7. Dry run
With `dryRun: true`, all validation runs, the GET lookups needed for validation are made,
and no PUT or PATCH is sent. The tool returns `{"dryRun":true,"method":...,"path":...,"body":...}`.

## Risks / Trade-offs

- [An agent actuates hardware unexpectedly] → Writes are off by default. The descriptions
  say so, the server instructions tell the model to confirm with the user, and every call
  is audited and can be dry-run first.
- [A state change is not effective immediately] → Metadata notifies the device service
  asynchronously, so a SET right after a LOCK can still succeed. The
  `set_device_admin_state` description says the change is eventually consistent.
- [A manually set DOWN persists] → The SDK only changes operatingState automatically when
  `AllowedFails > 0`, which defaults to 0. A DOWN stays until it is set back to UP, and the
  description says so.
- [Min/max checks differ from EdgeX when DataTransform=false] → Ours is stricter, which is
  acceptable.
- [No live verification yet] → The dev stack has not been run from the authoring
  environment. The `edgex-live-check` skill covers write tools only when explicitly asked.

## Verification notes

All checked against the source at the tags pinned in Phase 1 (edgex-go v4.0.2,
go-mod-core-contracts v4.0.3, device-sdk-go v4.0.2, device-virtual-go v4.0.2).

**SET: `PUT /api/v3/device/name/{name}/{command}` (core-command)**
- Route: https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/command/router.go (line 31).
- core-command decodes the body as a map and rejects empty, non-JSON or array bodies with
  400: https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/pkg/utils/http.go (226-245).
- SDK handling: https://github.com/edgexfoundry/device-sdk-go/blob/v4.0.2/internal/application/command.go
  - command or resource lookup (99-104)
  - readWrite `R` returns 405 (285-288, 351-354)
  - missing keys fall back to defaults, else 500 (291-299, 374-384)
  - value parsing with `fmt.Sprint`; Binary is unsupported (485-765)
  - Object normalization (779-825)
  - 423 lock/down checks (446-483)
- `ds-*` query params are stripped on SET:
  https://github.com/edgexfoundry/device-sdk-go/blob/v4.0.2/internal/controller/http/command.go (79, 123-140).
- min/max are enforced only when DataTransform is true:
  https://github.com/edgexfoundry/device-sdk-go/blob/v4.0.2/internal/transformer/transformparam.go (21-46).
- An event is published after a SET unless readWrite is `W`: `command.go` (94-97),
  `internal/common/utils.go` (53-81).
- The OpenAPI `SettingRequest` values are strings, and the spec does not list 405:
  https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/openapi/core-command.yaml (129-140, 652-722).

**State: `PATCH /api/v3/device` (core-metadata)**
- Route: https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/metadata/router.go (line 75).
- `UpdateDeviceRequest` requires `apiVersion`; `requestId` is optional and must be a UUID;
  `device` takes `name` or `id`; `adminState` is `oneof LOCKED UNLOCKED`; `operatingState`
  is `oneof UP DOWN UNKNOWN`:
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/requests/device.go (63-91)
  - https://github.com/edgexfoundry/go-mod-core-contracts/blob/v4.0.3/dtos/device.go (30-45)
- The response is 207 with a `BaseResponse` per item. A top-level 400 is returned when DTO
  validation fails during decode:
  https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/metadata/controller/http/device.go (187-223).
- The device service is notified asynchronously; LOCKED stops AutoEvents:
  - https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/internal/core/metadata/application/device.go (269-306)
  - https://github.com/edgexfoundry/device-sdk-go/blob/v4.0.2/internal/controller/messaging/callback.go (135-156)
- There is no dedicated adminstate or opstate route in API v3. A grep across edgex-go,
  go-mod-core-contracts, device-sdk-go and edgex-docs `odessa` found none.

**Secure mode:** the gateway locations `/core-command` and `/core-metadata` apply
`auth_request` with no method restriction, the same as reads:
https://github.com/edgexfoundry/edgex-go/blob/v4.0.2/cmd/security-proxy-setup/entrypoint.sh (223-246).

**MCP SDK v1.8.0:** `mcp.ToolAnnotations{ReadOnlyHint bool, DestructiveHint *bool,
IdempotentHint bool}`, as verified in Phase 1. No new SDK APIs are used.

## Open Questions

- Whether to add an optional per-device allowlist for writes (`--write-devices`). This is
  deferred until someone runs writes in production.
