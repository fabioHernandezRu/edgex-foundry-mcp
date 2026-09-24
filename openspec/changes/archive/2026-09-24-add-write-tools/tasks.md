## 1. EdgeX client

- [x] 1.1 Generalize the request helper to take a method and an optional JSON body (`Content-Type: application/json`), keeping timeouts, the no-redirect rule, name validation and error decoding
- [x] 1.2 Add `SetCommand` (PUT with a string-valued JSON object) and `UpdateDeviceState` (PATCH with a single-item array; 207 per-item status decoded into an `APIError`)
- [x] 1.3 Extend the fake EdgeX to record request bodies and content types, and to serve PUT (set commands; 405 for read-only, 423 when locked) and PATCH (207 per item; 404 for an unknown device)
- [x] 1.4 Tests: read methods issue only GET; exact write bodies and methods; 207 item failure; top-level 400

## 2. Validation

- [x] 2.1 Implement value validation per valueType (Int/Uint bit widths, Float, Bool, String, arrays as JSON text, Object as JSON, Binary rejected, empty non-String rejected) and min/max checks
- [x] 2.2 Table tests for every valueType family and boundary

## 3. Write tools (write set, `--enable-writes` only), each with a test and a README table row

- [x] 3.1 `set_device_command`: command lookup (`set: true`), exact parameter set, profile min/max, dry run, audit log, no retry; test; README row
- [x] 3.2 `set_device_admin_state`: LOCKED|UNLOCKED, dry run, audit log; test; README row
- [x] 3.3 `set_device_operating_state`: UP|DOWN|UNKNOWN, DOWN warning in the description, dry run, audit log; test; README row
- [x] 3.4 Registration tests: absent by default; present with `--enable-writes`; they pass `validateWriteTool`; the default-mode "no mutating request" test still holds

## 4. Docs

- [x] 4.1 README: write-tools table, safety model (dry run, audit, DOWN caveat, eventual consistency), roadmap update
- [x] 4.2 `edgex-live-check` skill: an opt-in section for write tools (dry run first; lock/unlock on device-virtual only)

## 5. Verification

- [x] 5.1 `make test lint`, `scripts/openspec_validate.sh` and `scripts/publish_guard.sh` are clean
- [x] 5.2 The `safety-reviewer` agent has reviewed the diff, and its findings are addressed
- [x] 5.3 Live check against EdgeX, if available; otherwise record that it was not run — NOT RUN: no Docker daemon in the authoring environment; all write paths tested against the fake EdgeX only
