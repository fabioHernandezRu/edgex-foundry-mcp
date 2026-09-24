## ADDED Requirements

### Requirement: Common tool output conventions
Every tool in this capability SHALL be **read-only** and SHALL be registered without
`--enable-writes`. Each tool SHALL declare typed input and output JSON schemas. It SHALL return structured
content with compact JSON that omits empty fields. Timestamps SHALL be RFC 3339 strings in
UTC (EdgeX `origin` nanoseconds and `created`/`modified` milliseconds converted). Items
SHALL be referenced by name rather than by EdgeX ID. List tools SHALL return `totalCount`
(when EdgeX reports it), `offset`, `count` and, when more items exist, `nextOffset`.
EdgeX errors SHALL be returned as MCP tool errors (`isError: true`) with a short, actionable
message. They SHALL NOT be returned as protocol errors.

#### Scenario: Not-found becomes a tool error
- **WHEN** `get_device` is called for a device name that core-metadata answers with 404
- **THEN** the tool result has `isError: true` and a message stating the device was not found, and suggesting `list_devices`

#### Scenario: EdgeX unreachable
- **WHEN** core-data is unreachable and `get_latest_readings` is called
- **THEN** the tool result has `isError: true` and a message naming core-data and the configured URL, without any token

### Requirement: system_health tool
The read-only `system_health` tool SHALL call `GET /api/v3/ping` and `GET /api/v3/version`
on core-metadata, core-data and core-command concurrently. For each service it SHALL report
`reachable`, `version`, `apiVersion`, `latencyMs`, and an `error` when unreachable. The
overall result SHALL include `healthy: true` only if all three services respond. The tool
takes no input.

#### Scenario: All services up
- **WHEN** all three fake services answer ping and version with 200
- **THEN** the result has `healthy: true`, and each service shows `reachable: true` and its `version`

#### Scenario: One service down
- **WHEN** core-command refuses connections
- **THEN** the tool does not fail as a whole, the result has `healthy: false`, core-command shows `reachable: false` with an error, and the other services are reported normally

### Requirement: list_device_services tool
The read-only `list_device_services` tool SHALL list device services from
`GET /api/v3/deviceservice/all`. Its inputs are optional `labels`, `offset` and `limit`
(default 20). Each item SHALL include `name`, `adminState`, `baseAddress`, `labels`,
`description`, and `properties` (redacted).

#### Scenario: List services
- **WHEN** the fake core-metadata returns one device service `device-virtual`
- **THEN** the result contains one item named `device-virtual` with its `adminState` and `baseAddress`

### Requirement: list_devices tool
The read-only `list_devices` tool SHALL list devices with optional filters `service`,
`profile` and `labels`, plus `offset` and `limit` (default 20). At most one of `service`,
`profile` and `labels` MAY be given, because EdgeX does not combine these filters
server-side. With `service`, it SHALL use `GET /api/v3/device/service/name/{name}`; with
`profile`, `GET /api/v3/device/profile/name/{name}`; otherwise `GET /api/v3/device/all`
(with `labels` if given). Each item SHALL include only `name`, `profile`, `service`,
`adminState`, `operatingState`, `labels` and `description`. It SHALL NOT include protocols
or properties.

#### Scenario: Filter by service
- **WHEN** `list_devices` is called with `service: "device-virtual"`
- **THEN** the fake core-metadata receives `GET /api/v3/device/service/name/device-virtual`, and the result lists its devices

#### Scenario: Conflicting filters
- **WHEN** `list_devices` is called with both `service` and `profile`
- **THEN** the tool returns `isError: true`, explaining that only one filter can be used, and makes no EdgeX request

#### Scenario: Unknown service yields empty list
- **WHEN** EdgeX returns 200 with an empty `devices` array for an unknown service
- **THEN** the result has `count: 0` and a hint that the service name may not exist

### Requirement: get_device tool
The read-only `get_device` tool SHALL return one device by `name` from
`GET /api/v3/device/name/{name}`. The output includes `name`, `description`, `profile`,
`service`, `adminState`, `operatingState`, `labels`, `location`, `autoEvents` (source name,
interval, onChange), `protocols` (redacted), `properties` (redacted), and
`created`/`modified`.

#### Scenario: Device returned with redacted protocols
- **WHEN** `get_device` is called for `Random-Integer-Device`
- **THEN** the result contains its profile, service, states and autoEvents, and any credential-like protocol property value is `***REDACTED***`

### Requirement: list_device_profiles tool
The read-only `list_device_profiles` tool SHALL list device profile summaries. Its inputs
are optional `manufacturer`, `model`, `labels`, `offset` and `limit` (default 20). With
`manufacturer` and/or `model`, it SHALL use the corresponding
`/api/v3/deviceprofile/manufacturer/...`, `/model/...` or
`/manufacturer/{m}/model/{m}` endpoint. Otherwise it SHALL use
`GET /api/v3/deviceprofile/basicinfo/all` (with `labels` if given). `labels` SHALL NOT be
combined with manufacturer/model. Each item SHALL include `name`, `manufacturer`, `model`,
`description`, `labels`, and `linkedDeviceCount` when available.

#### Scenario: Basic info listing
- **WHEN** `list_device_profiles` is called without filters
- **THEN** the fake core-metadata receives `GET /api/v3/deviceprofile/basicinfo/all`, and the result lists profile names with manufacturer and model

#### Scenario: Filter by manufacturer and model
- **WHEN** it is called with `manufacturer: "Example Corp"` and `model: "EX-1"`
- **THEN** the request path is `/api/v3/deviceprofile/manufacturer/Example%20Corp/model/EX-1`

### Requirement: get_device_profile tool
The read-only `get_device_profile` tool SHALL return one profile by `name` from
`GET /api/v3/deviceprofile/name/{name}`. The output includes the summary fields, plus
`resources` and `commands`. `resources` lists each non-hidden device resource with its
`name`, `description`, `valueType`, `readWrite`, `units`, `minimum`, `maximum`,
`defaultValue`, `mediaType` and redacted `attributes`. `commands` lists each non-hidden
device command with its `name`, `readWrite` and the resource names it operates on. The
optional input `includeHidden` (default false) SHALL include hidden resources and commands,
marked `hidden: true`.

#### Scenario: Profile with resources and commands
- **WHEN** `get_device_profile` is called for `Random-Integer-Device`
- **THEN** the result lists resources such as `Int8` with `valueType` `Int8` and `readWrite` `RW`, and lists the profile's device commands with their resource names

#### Scenario: Hidden resources omitted by default
- **WHEN** the profile contains a resource with `isHidden: true`
- **THEN** that resource is absent unless `includeHidden: true` is passed

### Requirement: get_latest_readings tool
The read-only `get_latest_readings` tool SHALL return the most recent readings of a
`device`, optionally restricted to one `resource`. `limit` defaults to 10. It SHALL use
`GET /api/v3/reading/device/name/{name}` or
`GET /api/v3/reading/device/name/{name}/resourceName/{resourceName}`, which EdgeX returns
newest first. Each reading SHALL include `resource`, `time` (RFC 3339), `valueType`,
`value`, and `units` when present. Binary readings follow the bounded-results rule of the
safety model. Object readings SHALL return `objectValue` as JSON.

#### Scenario: Latest readings for one resource
- **WHEN** it is called with `device: "Random-Integer-Device"`, `resource: "Int8"`, `limit: 3`
- **THEN** the fake core-data receives `.../reading/device/name/Random-Integer-Device/resourceName/Int8` with `limit=3`, and the result has at most 3 readings, newest first, with RFC 3339 times

#### Scenario: No readings
- **WHEN** EdgeX returns an empty readings array
- **THEN** the result has `count: 0` and a hint that the device may not be producing data or the name may be wrong

### Requirement: query_readings tool
The read-only `query_readings` tool SHALL return the readings of a `device` within a time
range, optionally restricted to one `resource`. The range is given either as `start` and
`end` (RFC 3339), or as `window` (a Go duration such as `15m` or `2h`, meaning "the window
ending now"). `offset` and `limit` (default 50) are supported. It SHALL use the
`.../start/{start}/end/{end}` reading endpoints with nanosecond bounds. `start` MUST be
before `end`, and exactly one of (`start`+`end`) or `window` MUST be given.

#### Scenario: Window query
- **WHEN** it is called with `device: "Random-Float-Device"`, `resource: "Float64"`, `window: "1h"`
- **THEN** the fake core-data receives `.../resourceName/Float64/start/<now-1h ns>/end/<now ns>`, and the result echoes the resolved `start` and `end` as RFC 3339

#### Scenario: Invalid range
- **WHEN** it is called with `start` after `end`
- **THEN** the tool returns `isError: true` and makes no EdgeX request

#### Scenario: Ambiguous range
- **WHEN** it is called with both `window` and `start`
- **THEN** the tool returns `isError: true`, explaining that the two forms are mutually exclusive

### Requirement: device_data_stats tool
The read-only `device_data_stats` tool SHALL report data volume for a `device`. It returns
`eventCount` (from `GET /api/v3/event/count/device/name/{name}`), `readingCount` (from
`GET /api/v3/reading/count/device/name/{name}`), and `lastReadingTime` with its `resource`
(from the newest reading, using limit 1). A device with no data SHALL yield counts of 0 and
no `lastReadingTime`, not an error.

#### Scenario: Stats for an active device
- **WHEN** EdgeX reports 12 events, 48 readings, and a newest reading at a known origin
- **THEN** the result is `eventCount: 12`, `readingCount: 48`, and `lastReadingTime` equal to that origin in RFC 3339

### Requirement: list_device_commands tool
The read-only `list_device_commands` tool SHALL list the core commands available for a
`device` via `GET /api/v3/device/name/{name}` on core-command. When `device` is omitted, it
lists devices with their commands via `GET /api/v3/device/all` (with `offset` and `limit`,
default 20). Each command SHALL include `name`, `get`, `set`, and `parameters` (resource
name and value type). The EdgeX-internal `url` SHALL be omitted.

#### Scenario: Commands of one device
- **WHEN** it is called with `device: "Random-Integer-Device"`
- **THEN** the result lists commands such as `Int8` with `get: true`, `set: true` and parameter `Int8`/`Int8`

### Requirement: read_device_command tool
The read-only `read_device_command` tool SHALL execute a core-command GET for a `device` and
`command`, and return the resulting readings. It SHALL use
`GET /api/v3/device/name/{device}/{command}` with `ds-pushevent=false`,
`ds-returnevent=true` and `ds-regexcmd=false`. Before calling, it SHALL verify through
core-command's device endpoint that the command exists and has `get: true`. If it does not,
it SHALL return a tool error listing the available GET commands. A 423 response SHALL be
reported as "device or service is locked or down". The tool is subject to
`--disable-device-reads` (see safety-model).

#### Scenario: Successful read
- **WHEN** it is called with `device: "Random-Integer-Device"` and `command: "Int8"`
- **THEN** the fake core-command receives the GET with the three safe query flags, and the result contains the event's readings in the common reading format

#### Scenario: Command without GET
- **WHEN** the requested command exists only with `set: true`
- **THEN** the tool returns `isError: true`, lists the GET-capable commands, and does not issue the command request

#### Scenario: Locked device
- **WHEN** core-command answers 423
- **THEN** the tool returns `isError: true` with a message that the device or its service is locked or down

### Requirement: Device profile resource template
The server SHALL expose the read-only MCP resource template
`edgex://deviceprofile/{name}` (MIME type `application/json`). Reading it returns the same
JSON document as `get_device_profile` with `includeHidden: true`. A URI whose name does not
exist SHALL yield the MCP resource-not-found error. Percent-encoded names SHALL be decoded.

#### Scenario: Read profile resource
- **WHEN** a client reads `edgex://deviceprofile/Random-Integer-Device`
- **THEN** it receives one JSON content item describing that profile's resources and commands

#### Scenario: Unknown profile resource
- **WHEN** a client reads `edgex://deviceprofile/does-not-exist` and core-metadata answers 404
- **THEN** the client receives a resource-not-found error
