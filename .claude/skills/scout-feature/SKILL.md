---
name: scout-feature
description: Use BEFORE proposing any new OpenSpec change for edgex-foundry-mcp that involves EdgeX endpoints or MCP SDK features (new tools, resources, transports, auth). Runs edgex-api-scout and mcp-sdk-scout in parallel, synthesizes endpoints, SDK APIs and risks, then hands off to openspec-new-change or openspec-ff-change. Writes no code.
---

# Scout a feature before proposing it

This skill produces evidence, not code. Its output feeds the proposal and `design.md`.

## 1. Frame the need

Write down in 3 to 6 bullets: the user-visible capability, the EdgeX data involved, read
vs write (safety model), and the target EdgeX version (default v4, API v3).

## 2. Launch both scouts IN PARALLEL (one message, two Agent calls)

The prompts must be self-contained, because the agents do not see this conversation.

- `edgex-api-scout`: list each operation needed, the filters and pagination required, and
  ask for secure-mode (API gateway) notes and any credential-bearing fields.
- `mcp-sdk-scout`: list the SDK features needed (e.g. typed tool handler with output
  schema, tool annotations, resources/templates, streamable HTTP handler, in-memory
  transport for tests). Ask for exact signatures at the pinned version.

## 3. Synthesize

Produce one section per tool or feature with:
- EdgeX endpoint(s), with source URLs and anything NOT VERIFIED
- SDK APIs used
- Output shape sketch (compact JSON) and limits
- Safety classification (read-only | --enable-writes) and redaction needs
- Risks and open questions (version drift, secure mode, large payloads, binary readings)

## 4. Hand off

- Small, well-understood change: `openspec-ff-change` (all artifacts in one go).
- Larger or uncertain change: `openspec-new-change`, then continue artifact by artifact.
- Paste the synthesis into `design.md` under "Verification notes". Do not write Go code
  in this skill.
