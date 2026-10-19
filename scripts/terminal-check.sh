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

{
  echo "Quotary terminal restoration check"
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "terminal: tmux PTY, 80x24, TERM=xterm-256color"
  run_case "quit-q" "q"
  run_case "quit-ctrl-c" "C-c"
} | tee docs/evidence/terminal-restoration.txt
