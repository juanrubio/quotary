#!/usr/bin/env bash
# Run the mandatory external review for one implementation phase.
# Usage: scripts/review-phase.sh <phase-number> <phase-name> [iteration]
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <phase-number> <phase-name> [iteration]" >&2
  exit 2
fi

phase="$1"
name="$2"
iteration="${3:-1}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
outdir="${TMPDIR:-/tmp}/quotary-reviews"
mkdir -p "$outdir"

prompt_file="$outdir/phase-${phase}-review-prompt.md"
output_file="$outdir/phase-${phase}-iter-${iteration}.md"

cat >"$prompt_file" <<EOF
You are an independent external code reviewer for the Quotary repository. You are
running as a separate Codex CLI process using GPT-6 Astra. You did not write the
changes under review.

Review Phase ${phase} — ${name} of docs/implementation-plan.md.

Read docs/implementation-plan.md (the phase, exit gate, traceability, and
definition of done), docs/specs/product.md, docs/specs/tui.md, the mockup guide,
and any ADR relevant to this phase.

Inspect all staged, unstaged, and untracked changes. Do not edit, create, delete,
stage, commit, or revert repository files. Do not access network resources,
credentials, cookies, or live provider accounts.

Verify with evidence:
1. Every task and exit-gate requirement for the phase is implemented.
2. Product and UX invariants are preserved: optional/null, freshness, source
   identity, demo isolation, and error handling.
3. Tests exercise required behavior and are not weakened, skipped, or vacuous.
4. Golden, capture, and fixture evidence is real and matches required scope.
5. No forbidden scope, no adapter details in the TUI, no color-only state.
6. Required build, vet, lint, docs-check, unit, and race checks pass.
7. Documentation and evidence are accurate. Unavailable live credentials may be
   open only where the plan permits; they are not a pass.

P0: unsafe/incorrect, spec invariant violated, build broken.
P1: missing phase requirement, weak/fake evidence, weakened test, exit gate unmet.
P2: non-blocking improvement.

Return exactly:
VERDICT: PASS | PASS WITH DOCUMENTED OPEN ITEMS | CHANGES REQUIRED | BLOCKED
PHASE: ${phase} — ${name}
SUMMARY: <2-5 sentences>
FINDINGS:
- [P0|P1|P2] <file>:<line> — <problem> — <why> — <required fix>
EVIDENCE CHECKED:
- <commands, files, tests, diff coverage>
OPEN ITEMS:
- <permitted open items or "none">
REQUIRED FIXES:
- <ordered, concrete actions; "none" only for PASS>
EOF

snapshot() {
  cd "$root"
  {
    git status --porcelain
    git diff --cached --binary
    git diff --binary
  } | sha256sum | cut -d' ' -f1
}

before="$(snapshot)"
cd "$root"
review_cache="${TMPDIR:-/tmp}/quotary-review-gocache"
mkdir -p "$review_cache"
GOCACHE="$review_cache" GOTMPDIR="${TMPDIR:-/tmp}" TMPDIR="${TMPDIR:-/tmp}" \
codex exec \
  --model gpt-6-astra \
  --ephemeral \
  --sandbox workspace-write \
  -C "$root" \
  --output-last-message "$output_file" \
  - <"$prompt_file"
after="$(snapshot)"

if [[ "$before" != "$after" ]]; then
  echo "reviewer changed the worktree; inspect before continuing" >&2
  exit 3
fi

echo "review output: $output_file"
