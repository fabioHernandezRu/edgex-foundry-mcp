# edgex-client Specification

## Purpose

The typed, read-only HTTP client for the EdgeX Foundry REST API v3 (core-metadata, core-data, core-command). It covers URL and query construction, limits, timeouts, the authorization header, and error decoding.

## Requirements
### Requirement: Typed read-only access to EdgeX core services
The `internal/edgex` package SHALL provide typed methods for the EdgeX REST API v3
endpoints that the read-only tools need, and no others:

- core-metadata: `GET /api/v3/ping`, `GET /api/v3/version`,
  `GET /api/v3/deviceservice/all`, `GET /api/v3/device/all`,
  `GET /api/v3/device/service/name/{name}`, `GET /api/v3/device/profile/name/{name}`,
  `GET /api/v3/device/name/{name}`, `GET /api/v3/deviceprofile/basicinfo/all`,
  `GET /api/v3/deviceprofile/manufacturer/{manufacturer}`,
  `GET /api/v3/deviceprofile/model/{model}`,
  `GET /api/v3/deviceprofile/manufacturer/{manufacturer}/model/{model}`,
  `GET /api/v3/deviceprofile/name/{name}`
- core-data: `GET /api/v3/ping`, `GET /api/v3/version`,
  `GET /api/v3/reading/device/name/{name}`,
  `GET /api/v3/reading/device/name/{name}/resourceName/{resourceName}`,
  `GET /api/v3/reading/device/name/{name}/start/{start}/end/{end}`,
  `GET /api/v3/reading/device/name/{name}/resourceName/{resourceName}/start/{start}/end/{end}`,
  `GET /api/v3/event/count/device/name/{name}`, `GET /api/v3/reading/count/device/name/{name}`
- core-command: `GET /api/v3/ping`, `GET /api/v3/version`, `GET /api/v3/device/all`,
  `GET /api/v3/device/name/{name}`, `GET /api/v3/device/name/{name}/{command}`

The client SHALL use only the HTTP GET method in this change.

#### Scenario: Only GET requests are issued
- **WHEN** every client method is exercised against a fake EdgeX server
- **THEN** the fake server observes only `GET` requests

#### Scenario: JSON decoding matches EdgeX DTO field names
- **WHEN** core-data returns `{"apiVersion":"v3","statusCode":200,"count":42}` for a reading count
- **THEN** the client returns a count of 42

### Requirement: Path and query construction
The client SHALL escape every path parameter with URL path escaping and build query
strings with proper encoding. Pagination SHALL be expressed with the EdgeX `offset` and
`limit` query parameters. Label filters SHALL be sent as a comma-separated `labels`
parameter. Time bounds SHALL be sent as Unix epoch nanoseconds.

#### Scenario: Device name with spaces and slashes
- **WHEN** a device named `Line 1/Sensor A` is requested
- **THEN** the request path contains `Line%201%2FSensor%20A` and no unescaped `/` inside the name segment

#### Scenario: Pagination and labels
- **WHEN** devices are listed with offset 20, limit 10, labels `site-a` and `floor-2`
- **THEN** the request query contains `offset=20`, `limit=10`, and `labels=site-a%2Cfloor-2` (or the equivalent encoded comma-separated value)

### Requirement: Limits never exceed the configured cap
The client SHALL never send `limit=-1`. It SHALL never send a `limit` larger than the
configured result cap, which is itself at most 1024 (the EdgeX default `MaxResultCount`,
above which EdgeX answers 400).

#### Scenario: Oversized limit is clamped
- **WHEN** a caller asks for limit 5000 with a configured cap of 100
- **THEN** the request carries `limit=100`

#### Scenario: Non-positive limit uses the default
- **WHEN** a caller passes limit 0 or a negative limit
- **THEN** the request carries the tool's documented default limit, not `-1`

### Requirement: Timeouts on every request
Every outbound request SHALL be bounded by the configured timeout, applied through the
request context, even when the caller's context has no deadline. The client SHALL NOT use
`http.DefaultClient`.

#### Scenario: Slow EdgeX service
- **WHEN** the fake EdgeX service sleeps longer than the configured timeout
- **THEN** the client returns a timeout error within roughly the configured timeout

### Requirement: Authorization header and TLS
When a token is configured, the client SHALL send `Authorization: Bearer <token>` on every
request. When a gateway CA file is configured, the client SHALL trust that CA in addition
to the system roots. The client SHALL NOT disable TLS verification.

#### Scenario: Bearer token attached
- **WHEN** a token is configured and any client method is called
- **THEN** the fake server receives the header `Authorization: Bearer <token>`

#### Scenario: No token configured
- **WHEN** no token is configured
- **THEN** requests carry no `Authorization` header

### Requirement: Error decoding
The client SHALL return a typed error for non-2xx responses. The error carries the HTTP
status code, the service name, and the EdgeX `message` when the body is an EdgeX
`BaseResponse` JSON. When the body is not JSON (for example, the plain-text
`request timeout` body EdgeX returns with 503), the error SHALL carry a truncated
plain-text excerpt of at most 200 bytes. Error strings SHALL NOT include request headers
or the token.

#### Scenario: EdgeX 404 with JSON body
- **WHEN** core-metadata answers 404 with `{"apiVersion":"v3","message":"no device with name 'X' found","statusCode":404}`
- **THEN** the returned error reports status 404, service `core-metadata`, and that message, and it is recognizable as not-found

#### Scenario: EdgeX 503 with plain-text body
- **WHEN** a service answers 503 with body `request timeout`
- **THEN** the returned error reports status 503 and the text `request timeout`

#### Scenario: 401 in secure mode
- **WHEN** the gateway answers 401
- **THEN** the returned error is recognizable as unauthorized, and its message suggests checking the token

### Requirement: Safe core-command GET parameters
When issuing a core-command GET for a device command, the client SHALL always send
`ds-pushevent=false`, `ds-returnevent=true` and `ds-regexcmd=false`. This way no event is
published to the message bus or persisted as a side effect, and the command name is never
interpreted as a regular expression over resource names.

#### Scenario: Read command query flags
- **WHEN** the client reads command `Int8` of device `Random-Integer-Device`
- **THEN** the request is `GET /api/v3/device/name/Random-Integer-Device/Int8` with query parameters `ds-pushevent=false`, `ds-returnevent=true` and `ds-regexcmd=false`

