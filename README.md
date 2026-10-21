# Quotary

**Your AI subscriptions, at a glance.**

Quotary shows the capacity, time left, reset times, and plans of the AI services you use, in one keyboard-driven terminal dashboard. It reads provider data through the [CodexBar](https://github.com/steipete/CodexBar) CLI installed on your machine, so your credentials stay with CodexBar — Quotary never copies them.

![Quotary's fictional subscription desk, showing twelve sources at once](docs/mockups/screens/01-subscription-desk.png)

*Fictional design reference. No real account or provider reading is shown here; terminal captures of the running app live in [docs/evidence/captures](docs/evidence/captures).*

**Status:** usable prototype. The offline demo, live CodexBar mode, scheduler, provider catalog, versioned preferences, and responsive TUI are implemented. Linux first.

## Install

There are no release binaries yet; build from source with Go 1.27.2 or newer:

```sh
make build
```

That produces `./quotary`. Move it somewhere on your `PATH` if you like.

For live mode you also need CodexBar: download it from its [releases page](https://github.com/steipete/CodexBar/releases), keep the `codexbar` executable beside its resource bundle, and set up your providers through CodexBar itself. Quotary finds the executable in this order: `QUOTARY_CODEXBAR_PATH`, the backend path configured in settings, then `codexbar` on `PATH`.

## Quick start

```sh
./quotary --demo   # offline fictional demonstration
./quotary          # live mode, reading through your CodexBar install
```

Demo mode never invokes CodexBar, reads provider credentials, or touches the network. On first launch, live mode seeds the enabled sources it can track automatically; press `p` to choose which sources to track. Preferences live at `$XDG_CONFIG_HOME/quotary/config.json` (falling back to `~/.config/quotary/config.json`) and are versioned and written atomically.

Inside the dashboard, everything is keyboard-driven and the footer shows the keys that matter right now. Press `?` any time for the full guide. The ones you will use most:

| Key | What it does |
|---|---|
| `↑↓←→` or `jk` | Move between sources |
| `Enter` | Open the selected source (Details) |
| `Tab` | Switch between Allowances and History, inside Details |
| `Esc` | Go back (asks before discarding unsaved settings changes) |
| `l` | Switch between cards and list |
| `/` | Search sources by name |
| `f` | Filter: all, needs attention, pinned |
| `s` | Change the sort order |
| `Space` | Pin or unpin a source; track or untrack it in the library |
| `r` / `R` | Refresh the selected source / every source |
| `p` | Open the source library, to add or remove sources |
| `,` | Open settings |
| `q` | Ask to quit; Cancel is selected by default |

## Good to know

- **What you see is what the backend reported.** Stale, missing, and genuinely exhausted readings stay distinct, and unrelated quotas are never combined into a made-up overall percentage. Where CodexBar does not report a plan — it happens — the card says "Plan not reported" rather than guessing.
- **Similar-looking sources stay separate.** Gemini app, Gemini CLI, and Antigravity are distinct sources, and API usage, credits, token history, and subscription allowances are never merged.
- **The catalog is a discovery surface.** CodexBar exposes 106 provider IDs; that makes them discoverable in the library, not proof of 106 working integrations.

## Preview the design

Open [`docs/mockups/index.html`](docs/mockups/index.html) in a browser. The interactive reference works locally without an account, backend, server, or network connection, and its keyboard controls work too. The [mockup implementer guide](docs/mockups/README.md) has the screen index, cell measurements, and instructions to export the reference screenshots again.

## Contributing — humans and AI tools welcome

Quotary itself was built with AI assistance, and contributions made that way are welcome here. Point your coding agent — or yourself — at [AGENTS.md](AGENTS.md): it has the map of the repo, how to build and verify, and the few rules that aren't obvious from the code. Contributions are judged on the change itself: specs respected, `make check` green. For anything bigger than a small fix, an issue first saves everyone time; [CONTRIBUTING.md](CONTRIBUTING.md) walks through the loop.

## Documentation and development

- [Product and technical specification](docs/specs/product.md) covers scope, the backend contract, data meaning, and acceptance criteria; the [visual and interaction specification](docs/specs/tui.md) covers layouts, keyboard behaviour, states, and themes.
- [Documentation index](docs/README.md) maps everything else: the [implementation plan](docs/implementation-plan.md), [architecture decisions](docs/decisions/), evidence and captures, and the [pinned CodexBar 0.77.0 provider catalog](docs/reference/codexbar-0.77.0-providers.json) the adapter was developed against.
- Hacking on it: `make check` runs vet, the pinned linter, docs-check, and the full offline test suite; `make build` builds the binary; `make race` adds the race detector for scheduler or process work.

## License

[MIT](LICENSE). CodexBar is a separate upstream project; its name and referenced behaviour remain subject to its own license and terms.
