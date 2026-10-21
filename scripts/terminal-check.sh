#!/usr/bin/env bash
# Prove that the TUI starts under a real PTY and restores the terminal on quit.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$root"

go build -o "$tmp/quotary" ./cmd/quotary
mkdir -p docs/evidence

cat >"$tmp/run.sh" <<'WRAPPER'
#!/bin/sh
stty -g >"$1/before.stty"
"$1/quotary" --demo
status=$?
stty -g >"$1/after.stty"
echo "QUOTARY_EXIT=$status"
if cmp -s "$1/before.stty" "$1/after.stty"; then
  echo "QUOTARY_STTY_RESTORED=1"
else
  echo "QUOTARY_STTY_RESTORED=0"
fi
sleep 5
WRAPPER
chmod +x "$tmp/run.sh"

cat >"$tmp/signal.sh" <<'WRAPPER'
#!/bin/sh
stty -g >"$1/before.stty"
"$1/quotary" --demo &
pid=$!
sleep 1
kill -TERM "$pid"
wait "$pid"
status=$?
stty -g >"$1/after.stty"
echo "QUOTARY_EXIT=$status"
if cmp -s "$1/before.stty" "$1/after.stty"; then
  echo "QUOTARY_STTY_RESTORED=1"
else
  echo "QUOTARY_STTY_RESTORED=0"
fi
sleep 5
WRAPPER
chmod +x "$tmp/signal.sh"

cat >"$tmp/fake-codexbar" <<'FAKE'
#!/bin/sh
case "$1" in
  --version) echo "CodexBar fake 0.77.0" ;;
  config) echo '{"providers":[{"provider":"codex","enabled":true,"defaultEnabled":true,"displayName":"Codex"}]}' ;;
  usage|cost) sleep 30 ;;
  *) echo "unsupported" >&2; exit 2 ;;
esac
FAKE
chmod +x "$tmp/fake-codexbar"
cat >"$tmp/hanging.json" <<JSON
{"version":1,"theme":"green","appearance":"auto","layout":"auto","tracked":["codex"],"pins":[],"intervalSeconds":300,"privacy":true,"backendPath":"$tmp/fake-codexbar"}
JSON
cat >"$tmp/hanging.sh" <<'WRAPPER'
#!/bin/sh
stty -g >"$1/before-hanging.stty"
QUOTARY_CONFIG_PATH="$1/hanging.json" "$1/quotary"
status=$?
stty -g >"$1/after-hanging.stty"
echo "QUOTARY_EXIT=$status"
if cmp -s "$1/before-hanging.stty" "$1/after-hanging.stty"; then echo "QUOTARY_STTY_RESTORED=1"; else echo "QUOTARY_STTY_RESTORED=0"; fi
sleep 5
WRAPPER
chmod +x "$tmp/hanging.sh"


run_case() {
  local label="$1"
  local key="$2"
  local session="quotary-check-$label"
  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x 80 -y 24 "sh $tmp/run.sh $tmp"
  sleep 1
  local before
  before="$(tmux capture-pane -p -t "$session" -S -30)"
  if ! grep -q "DEMO" <<<"$before"; then
    echo "$label: demo frame was not painted" >&2
    tmux kill-session -t "$session" 2>/dev/null || true
    exit 1
  fi
  tmux send-keys -t "$session" "$key"
  sleep 1
  local after
  after="$(tmux capture-pane -p -t "$session" -S -30)"
  tmux kill-session -t "$session" 2>/dev/null || true
  if ! grep -q "QUOTARY_EXIT=0" <<<"$after"; then
    echo "$label: application did not exit successfully" >&2
    printf '%s\n' "$after" >&2
    exit 1
  fi
  if ! grep -q "QUOTARY_STTY_RESTORED=1" <<<"$after"; then
    echo "$label: terminal settings were not restored" >&2
    printf '%s\n' "$after" >&2
    exit 1
  fi
  printf '%s: OK\n' "$label"
  printf -- '--- %s before quit ---\n%s\n--- %s after quit ---\n%s\n' "$label" "$before" "$label" "$after"
}

run_timeout_case() {
  local label="quit-hanging-backend"
  local session="quotary-check-$label"
  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x 80 -y 24 "sh $tmp/hanging.sh $tmp"
  sleep 2
  local before
  before="$(tmux capture-pane -p -t "$session" -S -30)"
  if ! grep -q "QUOTARY" <<<"$before"; then
    echo "$label: live shell was not painted while backend hung" >&2
    tmux kill-session -t "$session" 2>/dev/null || true
    exit 1
  fi
  tmux send-keys -t "$session" "qy"
  sleep 2
  local after
  after="$(tmux capture-pane -p -t "$session" -S -30)"
  tmux kill-session -t "$session" 2>/dev/null || true
  if ! grep -q "QUOTARY_EXIT=0" <<<"$after" || ! grep -q "QUOTARY_STTY_RESTORED=1" <<<"$after"; then
    echo "$label: timeout/hanging cleanup failed" >&2
    printf '%s\n' "$after" >&2
    exit 1
  fi
  printf '%s: OK\n' "$label"
  grep -E 'QUOTARY_EXIT|QUOTARY_STTY_RESTORED' <<<"$after"
}

run_signal_case() {
  local label="quit-sigterm"
  script -qec "sh $tmp/signal.sh $tmp" /dev/null > "$tmp/signal.out" 2>&1 || true
  if ! grep -q '?1049h' "$tmp/signal.out" || ! grep -q '?1049l' "$tmp/signal.out"; then
    echo "$label: signal case did not enter and leave the alternate screen" >&2
    cat "$tmp/signal.out" >&2
    exit 1
  fi
  if ! grep -q "QUOTARY_EXIT=0" "$tmp/signal.out"; then
    echo "$label: signal shutdown did not exit cleanly" >&2
    cat "$tmp/signal.out" >&2
    exit 1
  fi
  if ! grep -q "QUOTARY_STTY_RESTORED=1" "$tmp/signal.out"; then
    echo "$label: terminal settings were not restored" >&2
    cat "$tmp/signal.out" >&2
    exit 1
  fi
  printf '%s: OK\n' "$label"
  grep -E 'QUOTARY_EXIT|QUOTARY_STTY_RESTORED' "$tmp/signal.out"
}

{
  echo "Quotary terminal restoration check"
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "terminal: tmux PTY 80x24 TERM=xterm-256color for q/Ctrl+C/hanging-backend; script PTY for SIGTERM"
  run_case "quit-q" "qy"
  run_case "quit-ctrl-c" "C-c"
  run_signal_case
  run_timeout_case
} | tee docs/evidence/terminal-restoration.txt
