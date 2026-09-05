#!/usr/bin/env bash
# Launch ploi-tui against the local mock API, fully isolated — for promo
# screenshots/screencasts and offline development.
#
#   - starts mock/server.js on 127.0.0.1:8787 in the background; its request
#     log goes to a temp file, never to this terminal (it would corrupt the
#     TUI's screen)
#   - builds bin/ploi-tui if missing
#   - runs the TUI with throwaway XDG config/cache dirs and a dummy token,
#     so your real API token, config, and cache are never touched
#   - stops the TUI and the mock, and removes all temp files, when the TUI
#     exits or this script is interrupted (INT/TERM)
#
# Suggested terminal size for captures: >=74 columns for the wide layout,
# e.g. 130x38. Press ? in-app for keybindings.

set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v node >/dev/null 2>&1; then
  echo "error: node is required to run the mock API" >&2
  exit 1
fi

PORT="${PORT:-8787}"
HOST="${HOST:-127.0.0.1}"

if (exec 3<>"/dev/tcp/${HOST}/${PORT}") 2>/dev/null; then
  exec 3>&- 3<&- || true
  echo "error: ${HOST}:${PORT} is already in use (is another mock running?)" >&2
  exit 1
fi

if [ ! -x bin/ploi-tui ]; then
  if command -v make >/dev/null 2>&1; then
    make build
  else
    CGO_ENABLED=0 go build -o bin/ploi-tui ./cmd/ploi-tui
  fi
fi

mock_log="$(mktemp)"
node mock/server.js >"$mock_log" 2>&1 &
mock_pid=$!
work=""
tui_pid=""

cleanup() {
  if [ -n "$tui_pid" ]; then
    kill "$tui_pid" 2>/dev/null || true
  fi
  kill "$mock_pid" 2>/dev/null || true
  if [ -n "$work" ]; then
    rm -rf "$work"
  fi
  rm -f "$mock_log"
}
trap cleanup EXIT INT TERM

# Wait until the mock accepts connections.
for _ in $(seq 1 50); do
  if node -e '
    require("http").get("'"http://${HOST}:${PORT}"'/api/user", (r) => process.exit(r.statusCode === 200 ? 0 : 1))
      .on("error", () => process.exit(1));
  ' 2>/dev/null; then
    break
  fi
  sleep 0.1
done

work="$(mktemp -d)"
mkdir -p "$work/config/ploi-tui"
printf 'api_token = "local-mock-token"\n' > "$work/config/ploi-tui/config.toml"

echo "Mock API request log: $mock_log (removed when the TUI exits)"

XDG_CONFIG_HOME="$work/config" XDG_CACHE_HOME="$work/cache" \
  PLOI_TUI_API_URL="http://${HOST}:${PORT}" ./bin/ploi-tui &
tui_pid=$!
wait "$tui_pid"
