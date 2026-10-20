# Phase 8 live validation record

Date: 2026-10-20 (UTC)

## Environment and executable resolution

- `QUOTARY_CODEXBAR_PATH`: unset.
- Quotary configuration at the default XDG path: absent, so no configured backend
  path exists.
- `command -v codexbar`: empty with nonzero exit; no `codexbar` entry exists in
  the checked executable directories.
- Detected CodexBar version: unavailable.
- Credentials, cookies, tokens, and raw provider payloads: not accessed.

The executable-resolution order is environment override → configured path →
`PATH`; all three routes produced no executable, so no live command was run.

## Priority cases

| Case | Provider ID | Intended usage command | Intended history command | Result | Native-only | Contract fixture coverage |
| --- | --- | --- | --- | --- | --- | --- |
| Claude Code | `claude` | `usage --provider claude --format json` | `cost --provider claude --format json --days 7 --provider-native-only` | unavailable: no executable | flag not probed | `TestUsageWindowsCreditsDetailsAndMultiAccount`, `TestHistoryParsesDaysModelsCostAndPartial`, `TestNativeOnlyFallbackDoesNotRetryFatalErrors` |
| Codex | `codex` | `usage --provider codex --format json` | `cost --provider codex --format json --days 7 --provider-native-only` | unavailable: no executable | flag not probed | `TestUsageWindowsCreditsDetailsAndMultiAccount`, `TestUsageExpandsEveryAccount`, `TestHistoryFallsBackWhenNativeOnlyUnavailable` |
| Muse Code | `muse` | `usage --provider muse --format json` | `cost --provider muse --format json --days 7` | unavailable: no executable | not applicable | demo-only token-only: `TestMuseHistoryIsTokenOnly`; adapter: `TestHistoryRepeatedModelsAccumulate`, `TestMissingCurrencyIsNotInventedAndPricingCoverageDisclosed`, `TestDeclaredNullWindowPreservedAsUnavailable` |
| Google · Gemini app | `gemini` (`web`) | `usage --provider gemini --source web --format json` | none promised | unavailable: no executable; Linux cookie setup unverified | not applicable | `TestUsagePassesOnlyExplicitValidatedSource`, `TestApplyRetiresGeminiGenericAndSplitsVariants` |
| Google · Gemini CLI | `gemini` (`api`) | `usage --provider gemini --source api --format json` | none promised | unavailable: no executable | not applicable | `TestUsagePassesOnlyExplicitValidatedSource`, `TestApplyRetiresGeminiGenericAndSplitsVariants` |
| Google · Antigravity | `antigravity` | `usage --provider antigravity --format json` | `cost --provider antigravity --format json --days 7` | unavailable: no executable | not applicable | `TestUsageWindowsCreditsDetailsAndMultiAccount`, `TestGroupedWindowKeysAreStableAcrossOrder`, `TestGroupedExplicitWindowKeysStableAcrossOrder` |

## Additional subscription cases

| Case | Provider ID | Intended usage command | Result | Native-only | Contract fixture coverage |
| --- | --- | --- | --- | --- | --- |
| Cursor | `cursor` | `usage --provider cursor --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| GitHub Copilot | `copilot` | `usage --provider copilot --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Windsurf | `windsurf` | `usage --provider windsurf --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Kimi Code | `kimi` | `usage --provider kimi --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Kiro | `kiro` | `usage --provider kiro --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| JetBrains AI | `jetbrains` | `usage --provider jetbrains --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Zed | `zed` | `usage --provider zed --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Warp | `warp` | `usage --provider warp --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Augment | `augment` | `usage --provider augment --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Kilo | `kilo` | `usage --provider kilo --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Droid | `factory` | `usage --provider factory --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| OpenCode Go | `opencodego` | `usage --provider opencodego --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| z.ai | `zai` | `usage --provider zai --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| MiniMax | `minimax` | `usage --provider minimax --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| Perplexity | `perplexity` | `usage --provider perplexity --format json` | unavailable: no executable | not applicable | Catalog bulk eligibility: `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |

## Catalog coverage (separate from live verification)

The reviewed 106-provider manifest and per-provider classifications are recorded
in `docs/evidence/catalog-review.md`. Azure OpenAI is the sole active probe and is
outside the prototype. Catalog coverage is not live verification.

## Fixture/contract coverage (separate from live verification)

Adapter contract tests cover windows, credits, money, grouped windows, histories,
error taxonomy, variants, and multi-account shapes under `internal/backend/codexbar`.
Those fixtures prove normalization and command construction, not provider
entitlement or live CLI behavior.

## Setup limits

- Linux automatic browser-cookie import is not part of this baseline; Gemini app
  may require explicit cookie setup through CodexBar.
- Muse's macOS Keychain fallback is unavailable on Linux; its CLI-owned auth file
  may be usable when present.
- Azure OpenAI remains an active/inference probe and is outside the prototype.
- A registered provider does not prove this machine can authenticate it.

## Conclusion

No live success is claimed. Every priority and additional case is recorded as
unavailable, not passed. Native-only history availability for Claude/Codex was
not probed. Re-run this record with an installed version-pinned CodexBar and real
credentials before claiming live verification in Phase 9.
