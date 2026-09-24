---
name: add-mcp-tool
description: Use when adding or changing a single MCP tool (or resource) in edgex-foundry-mcp. It is the end-to-end recipe covering the OpenSpec task, endpoint verification, typed EdgeX client method, handler with read/write registration and redaction, httptest fixture, table test, README row, and ticking tasks.md. Do not use for changes that touch no tool.
---

# Add an MCP tool end-to-end

Follow these steps in order. Do not skip a step because the tool "looks simple".

## 1. Confirm an OpenSpec change covers it

- `openspec list` and open the active change's `tasks.md`. There must be a task for this
  tool and a requirement plus scenario for it in the change's `specs/`.
- If not, STOP and create or extend a change first (`openspec-new-change`, or
  `scout-feature` for anything non-trivial). No production code outside a change.

## 2. Verify the endpoint

- Launch the `edgex-api-scout` agent with a self-contained prompt: service, operation,
  filters needed, and target EdgeX version (v4, API v3).
- Copy the verified method, path, params and response shape, with source URLs, into the
  change's `design.md` under "Verification notes". Anything marked NOT VERIFIED blocks
  coding.
- For an SDK question, launch `mcp-sdk-scout`.

## 3. Typed client method (`internal/edgex`)

- Add response DTO structs with only the fields the tool needs. JSON tags must match the
  spec exactly.
- The method takes `context.Context` and typed params, escapes path segments with
  `url.PathEscape`, builds the query with `url.Values`, and uses the shared `do` helper
  (timeout, auth header, error decoding).
- Never log request headers. Return a typed error containing the EdgeX `message` and
  status code.

## 4. Handler (`internal/tools`)

- Define typed `In` and `Out` structs with `json` and `jsonschema` tags. Descriptions are
  written for an LLM: what it does, when to use it, units, and limits.
- **Read tool:** register it in the read-only set with the `ReadOnlyHint: true`
  annotation.
- **Write tool:** register it ONLY inside the `--enable-writes` branch. Its description
  must contain "This affects physical hardware" (or "modifies EdgeX metadata"). Set
  `DestructiveHint` where appropriate.
- Clamp `limit` to the package max. Never send `limit=-1`.
- Pass device protocols and any free-form property maps through the redaction helper
  before returning them.
- Output compact JSON: drop empty fields, prefer names over IDs, and include `truncated`
  or `nextOffset` when capped.

## 5. Fixture and tests

- Add `internal/tools/testdata/<tool>.json` (or edgex testdata), modeled on the official
  example payload from step 2. Use neutral names (`Random-Integer-Device`, `site-a`) and
  RFC 5737 IPs.
- Write a table-driven test against an `httptest.Server` fake. Cover: happy path, not
  found (404), EdgeX error body, limit clamping, and redaction (a protocol property named
  like a password comes back as `***`). For write tools, also assert the tool is NOT
  registered without `--enable-writes`.
- `make test lint` must be green.

## 6. Docs and bookkeeping

- Add or update the row in the README tool table: name, read or write, EdgeX endpoint,
  and a one-line purpose.
- Tick the task in `tasks.md` (`- [x]`).
- `openspec validate <change> --strict` must be clean.
- If a live EdgeX is available, run the `edgex-live-check` skill for this tool.
