# Mock ploi.io API (temporary, for promo screenshots)

A zero-dependency Node.js mock of the ploi.io API that serves **100% fictional
data** so screenshots never expose real servers, IPs, or domains.

Everything here is throwaway — delete the `mock/` folder when you're done.
No application code is involved or modified.

## Usage

Terminal 1 — start the mock (listens on `127.0.0.1:8787` only):

```bash
node mock/server.js            # custom port: PORT=9000 node mock/server.js
```

Terminal 2 — run the TUI against it, with an isolated cache so the real
account's cache never leaks in (and the mock's data isn't written to yours):

```bash
XDG_CACHE_HOME="$(mktemp -d)" PLOI_TUI_API_URL=http://127.0.0.1:8787 ./bin/ploi-tui
```

`PLOI_TUI_API_URL` is the built-in API override in `internal/api/client.go`;
no code changes needed.

## Fictional data

| Server                 | IP            | Status      | Notes                          |
| ---------------------- | ------------- | ----------- | ------------------------------ |
| `northstar-production` | `203.0.113.10`| active      | 3 sites, monitoring, DBs       |
| `northstar-staging`    | `203.0.113.27`| active      | 2 sites, deploying site        |
| `edge-worker-04`       | `203.0.113.42`| rebooting   | monitoring-unavailable state   |

Domains use `example.io` / `example.com` / `example.dev` / `example.net`;
IPs use `203.0.113.x` (reserved for documentation, RFC 5737). Monitoring data
is deterministic, so screenshots are reproducible. Certificate expiry dates are
generated relative to "now" to show OK / warning / critical states.

## Endpoints mocked

- `GET /api/user`
- `GET /api/servers`, `GET /api/servers/:id`
- `GET /api/servers/:id/databases`
- `GET /api/servers/:id/monitor`
- `GET /api/servers/:id/system-users`
- `GET /api/servers/:id/sites`, `GET /api/servers/:id/sites/:siteId`
- `GET /api/servers/:id/sites/:siteId/certificates`
- `POST /api/servers/:id/restart` (flips the fixture to `rebooting`)

## Safety notes

- Binds to `127.0.0.1` only — never exposed to the network.
- Logs request paths only; never logs headers or your API token.
- The TUI still sends its real token to the mock, so don't point anything
  else at it and don't run it longer than needed.

## Cleanup

Stop the server (`Ctrl+C`) and delete the folder:

```bash
rm -rf mock/
```

## Local development and testing

The same mock works as an offline backend while developing or debugging the
app — handy when you need reproducible API responses, want to avoid burning
real rate limit, or have no connectivity.

Start the mock and run the app from source against it:

```bash
node mock/server.js
PLOI_TUI_API_URL=http://127.0.0.1:8787 go run ./cmd/ploi-tui
```

For a fully isolated environment (own token store, own cache), point the XDG
directories at a throwaway location. The mock does not validate tokens, so a
dummy value is enough:

```bash
T="$(mktemp -d)" && mkdir -p "$T/config/ploi-tui"
printf 'api_token = "local-mock-token"\n' > "$T/config/ploi-tui/config.toml"
XDG_CONFIG_HOME="$T/config" XDG_CACHE_HOME="$T/cache" \
  PLOI_TUI_API_URL=http://127.0.0.1:8787 ./bin/ploi-tui
```

Tips:

- Every request is logged to the mock's terminal — useful for seeing exactly
  which endpoints a screen or action hits (paths only, no headers).
- All fixtures live at the top of `mock/server.js`. Edit them to exercise
  different UI states: statuses (`active`, `rebooting`, `deploying`),
  empty lists, monitoring on/off, certificate expiry windows, etc.
- `POST /api/servers/:id/restart` flips the fixture to `rebooting`, so you
  can walk through the restart confirmation and status-poll flow offline.
- Requests for unknown server/site IDs return `404`, which is an easy way to
  check error handling paths.
- Monitoring data is deterministic, so charts look identical on every run —
  useful when comparing UI changes.
- Custom port: `PORT=9000 node mock/server.js`, then use that port in
  `PLOI_TUI_API_URL`.
