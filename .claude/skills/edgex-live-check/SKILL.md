---
name: edgex-live-check
description: Use to validate edgex-foundry-mcp behavior against a RUNNING EdgeX Foundry instance (default is the dev/docker-compose stack on localhost) after implementing or changing tools, before calling a change done, or when the user asks to "try it for real". Checks service ping/version, devices and device-virtual readings, then exercises the MCP tools. Read-only unless the user explicitly asks to test write tools.
---

# Live check against a running EdgeX

Default target: the `dev/` stack (non-secure mode) on `localhost`. For another target, the
user must give the URLs. Never hard-code or commit them.

**Read-only by default.** Do not start the server with `--enable-writes`, and do not call
any PUT/POST/PATCH/DELETE endpoint or core-command SET, unless the user explicitly asks
for it in this conversation.

## 1. Stack up and the RIGHT build running

```sh
make dev-up                                      # or: docker compose -f dev/docker-compose.yml up -d
docker compose -f dev/docker-compose.yml ps
for p in 59880 59881 59882; do curl -s http://localhost:$p/api/v3/ping; echo; done
for p in 59880 59881 59882; do curl -s http://localhost:$p/api/v3/version; echo; done
```

Check that `version` matches the image tags pinned in `dev/docker-compose.yml`. A stale
container from an older compose file gives false positives. If it does not match, run
`docker compose -f dev/docker-compose.yml up -d --force-recreate`.

## 2. EdgeX data is flowing

```sh
curl -s "http://localhost:59881/api/v3/deviceservice/all?limit=10"
curl -s "http://localhost:59881/api/v3/device/all?limit=10"
curl -s "http://localhost:59880/api/v3/reading/device/name/Random-Integer-Device?limit=5"
curl -s "http://localhost:59882/api/v3/device/name/Random-Integer-Device"
```

device-virtual should list `Random-*-Device` devices, and readings should be recent.

## 3. Exercise the MCP server

- Build the current tree: `make build`.
- Drive it with the helper client (stdio), for example:
  `go run ./scripts/mcpcall -- tools/list` and
  `go run ./scripts/mcpcall -- call get_latest_readings '{"device":"Random-Integer-Device","limit":3}'`.
- Or use the MCP Inspector: `npx @modelcontextprotocol/inspector ./bin/edgex-foundry-mcp`.
- For each tool in the change: record the input, a trimmed output, and whether it matches
  the spec scenario. Confirm that no protocol credentials or tokens appear in the output
  or in stderr logs.

## 4. Report

Give a short table (tool, input, result, spec scenario matched: yes/no), plus the
`/api/v3/version` output you verified against. Never paste tokens.

## Troubleshooting

| Symptom | Likely cause | Fix |
|--------|--------------|-----|
| `ping` refused on a port | Service not started or still starting | `docker compose ... ps`, then `logs <service>` |
| `device/all` empty | device-virtual not registered yet, or its profiles failed to load | Check `logs device-virtual`, wait about 30 s, re-query `deviceservice/all` |
| Readings empty | Auto-events not running, or core-data persistence disabled | Check device-virtual logs and `autoEvents` in the device definition |
| Versions differ from compose tags | Stale image or container | `pull` then `up -d --force-recreate` |
| HTTP 401 on every call | Secure-mode EdgeX | Use the API gateway URL plus a JWT (`--gateway-url`, `EDGEX_TOKEN` env). Direct service ports are not reachable with auth off. |
| 404 on `/api/v3/...` | Pre-v3 EdgeX (Jakarta-era v2) or wrong base URL | Confirm `/api/v3/version`. This server targets API v3 only. |
| MCP server exits immediately | Bad flags or config validation | Run the binary with `--help`, and read stderr |
