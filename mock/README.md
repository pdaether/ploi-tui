# Mock ploi.io API (temporary, for promo screenshots)

A zero-dependency Node.js mock of the ploi.io API that serves **100% fictional
data** so screenshots never expose real servers, IPs, or domains.

Everything here is throwaway — delete the `mock/` folder when you're done.
No application code is involved or modified.

## Usage

One command — starts the mock, builds the binary if needed, and launches the
TUI fully isolated (throwaway config/cache, dummy token; your real account is
never touched):

```bash
./mock/demo.sh
```

Or manually. Terminal 1 — start the mock (listens on `127.0.0.1:8787` only):

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

## Fictional fleet

30 fictional servers, grouped like a believable company fleet, each with 1–3
sites (45 sites in total):

| Group              | Servers                                  | Notes                                             |
| ------------------ | ---------------------------------------- | ------------------------------------------------- |
| `northstar-*`      | production, staging, edge-worker-04      | flagship Laravel apps; edge-worker-04 **rebooting** |
| `atlas-*`          | api-prod-01/02, api-staging, docs        | load-balanced API pair; staging runs PHP 8.4      |
| `horizon-web-*`    | prod-01/02, staging                      | web frontends, mirrored domains behind an LB      |
| `lumen-worker-*`   | worker-01..04                            | queue workers with tiny status sites              |
| `beacon-db-prod-*` | db-prod-01/02                            | database nodes (MySQL 8.0/8.4, Adminer site)      |
| `relay-edge-*`     | edge-01..04                              | edge/CDN static sites; edge-01 **building**       |
| `quest-*`          | store-prod, store-staging, blog          | WordPress shops                                   |
| `compass-admin`    | —                                        | internal tools (admin/tracker/wiki)               |
| one-offs           | sentinel-monitoring, aurora-analytics, ember-cron, kestrel-mail, harbor-backup, meridian-sandbox | kestrel-mail **refreshing**, harbor-backup **unreachable**, meridian has a deploy-failed site |

Status mix: 26 `active` + 4 non-active (`rebooting`, `building`,
`refreshing`, `unreachable`) so the colored status badges show up.

All IPs come from `203.0.113.x` (reserved for documentation, RFC 5737) and all
domains from `example.{io,com,net,dev}`. Monitoring data is deterministic, so
screenshots are reproducible. Certificate expiry dates are spread across OK /
warning (<=30d) / critical (<=7d), and `last_deploy_at` ranges from "minutes
ago" to "weeks ago" (plus never-deployed sites).

## Endpoints mocked

- `GET /api/user`
- `GET /api/servers`, `GET /api/servers/:id`
- `GET /api/servers/:id/databases`
- `GET /api/servers/:id/monitor`
- `GET /api/servers/:id/system-users`
- `GET /api/servers/:id/sites`, `GET /api/servers/:id/sites/:siteId`
- `GET /api/servers/:id/sites/:siteId/certificates`
- `POST /api/servers/:id/restart` (flips the fixture to `rebooting`)

## Capturing screenshots / screencast

Use a terminal of at least 74 columns so the wide list layout kicks in —
130x38 is a comfortable size. Press `?` in-app to see all keybindings.

A suggested ~60s walkthrough:

1. Server list — scroll with `j`/`k` (or arrows), showing status badges and
   the quota in the header
2. `/` to filter, type `prod`, `enter` — then `esc esc` to clear
3. `enter` on `northstar-production` — Overview tab with quick stats
4. `2` — Monitoring tab (24h charts); `3` — Sites tab
5. `enter` on a site — detail with certificate expiry colors
6. `esc` `esc` back to the list
7. Optional action shots: `ctrl+r` on a server detail, `y` to confirm the
   restart (mock flips to `rebooting`); `s` for the SSH user picker;
   `?` for the help overlay

If a key does something different in-app (the UI may have evolved), just
follow the on-screen hints — the data is the same either way.

## Safety notes

- Binds to `127.0.0.1` only — never exposed to the network.
- Logs request paths only; never logs headers or your API token.
- When using `demo.sh`, only a dummy token (`local-mock-token`) ever reaches
  the mock. If you run the TUI manually against it, it sends your real token
  — so don't point anything else at the mock and don't run it longer than
  needed.

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
- All fixtures live at the top of `mock/server.js`. Each server is one
  `addServer({...})` call; edit or add entries to exercise different UI
  states: statuses (`active`, `building`, `rebooting`, `refreshing`,
  `unreachable`, `deploying`, `deploy-failed`), monitoring on/off,
  certificate expiry windows, empty lists, etc.
- `POST /api/servers/:id/restart` flips the fixture to `rebooting`, so you
  can walk through the restart confirmation and status-poll flow offline.
- Requests for unknown server/site IDs return `404`, which is an easy way to
  check error handling paths.
- Monitoring data is deterministic, so charts look identical on every run —
  useful when comparing UI changes.
- Custom port: `PORT=9000 node mock/server.js` (or `PORT=9000 ./mock/demo.sh`),
  then use that port in `PLOI_TUI_API_URL`.
