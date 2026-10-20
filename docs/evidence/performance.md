# Performance and responsiveness measurements

Date: 2026-10-20
Host: Linux amd64, Go 1.27.2, model-independent local measurement

## Demo first-paint render cost

Methodology: `go test ./internal/tui -run TestFirstPaintBudget -count=1 -v` constructs the demo session/model and renders the 160×48 first frame 20 times, then reports the average. The guard fails above 100 ms.

Result: **19.2 ms average** in the closeout run (independent runs vary between
roughly 18–20 ms because this is local wall-clock measurement). `BenchmarkFirstPaint` reports approximately **13.6 ms/op** for repeated first paints on the same host.

## Responsiveness during work

- 108-source scheduler-backed UI stress: `TestSchedulerBackedTUIStaysResponsiveWith108Sources` renders and navigates 50 iterations while fetches run, under a 2 s guard, and requires all 108 sources to reach Current.
- Fetch/search/scroll/resize interaction coverage: `TestSearchAndFilterContext`, `TestHistoryScrollReachesLastDailyValue`, `TestDetailsMetadataAtNarrowWidths`, `TestResponsiveHeightUsesCompactLayout`, `TestAllScreensRenderAtRequiredSizes`.
- Live local countdown/age updates without backend traffic: `TestLiveLocalTickAdvancesAgeWithoutBackendTraffic`.
- Scheduler concurrency ceiling: `TestConcurrencyCeilingAndStress`.

Live first-paint render cost: `TestLiveFirstPaintBudgetDespiteSlowDiscovery` starts
its timer before `live.New`, uses a channel-blocked catalog backend so discovery
cannot complete, then requires `live.New` + model construction + first `View()` in
under 100 ms. Discovery is asynchronous and the shell paints first; the rendered
catalog/version updates on a live event.

These are local engineering measurements, not release performance certification.
