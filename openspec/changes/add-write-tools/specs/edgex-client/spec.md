## MODIFIED Requirements

### Requirement: Typed read-only access to EdgeX core services
The `internal/edgex` package SHALL provide typed methods for the EdgeX REST API v3
endpoints that the tools need, and no others:

- core-metadata: `GET /api/v3/ping`, `GET /api/v3/version`,
  `GET /api/v3/deviceservice/all`, `GET /api/v3/device/all`,
  `GET /api/v3/device/service/name/{name}`, `GET /api/v3/device/profile/name/{name}`,
  `GET /api/v3/device/name/{name}`, `GET /api/v3/deviceprofile/basicinfo/all`,
  `GET /api/v3/deviceprofile/manufacturer/{manufacturer}`,
  `GET /api/v3/deviceprofile/model/{model}`,
  `GET /api/v3/deviceprofile/manufacturer/{manufacturer}/model/{model}`,
  `GET /api/v3/deviceprofile/name/{name}`, and (write) `PATCH /api/v3/device`
- core-data: `GET /api/v3/ping`, `GET /api/v3/version`,
  `GET /api/v3/reading/device/name/{name}`,
  `GET /api/v3/reading/device/name/{name}/resourceName/{resourceName}`,
  `GET /api/v3/reading/device/name/{name}/start/{start}/end/{end}`,
  `GET /api/v3/reading/device/name/{name}/resourceName/{resourceName}/start/{start}/end/{end}`,
  `GET /api/v3/event/count/device/name/{name}`, `GET /api/v3/reading/count/device/name/{name}`
- core-command: `GET /api/v3/ping`, `GET /api/v3/version`, `GET /api/v3/device/all`,
  `GET /api/v3/device/name/{name}`, `GET /api/v3/device/name/{name}/{command}`, and (write)
  `PUT /api/v3/device/name/{name}/{command}`

The client SHALL use HTTP GET for every read method. PUT and PATCH SHALL be used only by
the two write methods (`SetCommand`, `UpdateDeviceState`). Those methods SHALL be called
only from write tools.

#### Scenario: Only GET requests are issued by read methods
- **WHEN** every read method is exercised against a fake EdgeX server
- **THEN** the fake server observes only `GET` requests

#### Scenario: JSON decoding matches EdgeX DTO field names
- **WHEN** core-data returns `{"apiVersion":"v3","statusCode":200,"count":42}` for a reading count
- **THEN** the client returns a count of 42

#### Scenario: Write methods use the verified methods and bodies
- **WHEN** `SetCommand` and `UpdateDeviceState` are called
- **THEN** the fake EdgeX observes `PUT` with a JSON object body and `PATCH` with a single-item JSON array body respectively, both with `Content-Type: application/json`
