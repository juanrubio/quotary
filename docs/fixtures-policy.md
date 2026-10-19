# Quotary fixtures policy

`testdata/codexbar/` contains synthetic or deliberately redacted contract
fixtures for the CodexBar adapter.

Allowed:

- hand-written JSON approved by the implementer
- redacted provider payloads with names, accounts, emails, project paths,
  tokens, cookies, and request identifiers removed
- failure fixtures for malformed, oversized, partial, and incompatible data

Forbidden:

- credentials, cookies, API keys, session tokens, or authentication URLs
- account emails, real names, workspace names, or local project paths
- raw live payloads captured without review
- fixture files that cannot be committed to a public repository

Every fixture family must have a matching test in
`internal/backend/codexbar`.
