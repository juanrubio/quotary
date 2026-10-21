# VHS tapes

Recorded terminal demonstrations live here. Tapes use the deterministic demo
backend and fictional data only.

## Showcase

`showcase.tape` is the read-only tour embedded in the README: the
subscription desk, a source's details and history, the compact list, the
source library, the keyboard guide, and the quit confirmation. Regenerate
it from the repository root with a built binary (VHS also needs `ttyd` and
`ffmpeg`):

```sh
make build
cd tapes && vhs showcase.tape   # writes ../docs/media/showcase.gif
```

Review the GIF before committing it; it must never show a real account.

Add one tape per walkthrough, for example:

```text
01-demo-overview.tape
02-details-and-history.tape
03-source-library.tape
04-attention-states.tape
```

Do not record live credentials, account labels, or real provider readings.
