# Biloba vs Playwright — a performance comparison

> 🤖 **This whole comparison — harness, scenarios, charts, and write-up — was generated
> by [Claude](https://www.anthropic.com/claude) (Opus 4.8), working from a brief.
> Feedback is welcome, just [open an issue](https://github.com/onsi/biloba-comparison/issues)!**

A fair, reproducible **speed** comparison between
[Biloba](https://github.com/onsi/biloba) (Go, on chromedp) and
[Playwright Test](https://playwright.dev) (TypeScript/Node). One shared server, one
canonical scenario list implemented identically on every side, a framework-neutral
external stopwatch. The aim is to measure framework + runtime overhead for an
identical browser-test workload — not to manufacture a win. Read
[`METHODOLOGY.md`](./METHODOLOGY.md) for the fairness controls and threats-to-validity,
and [`SCENARIOS.md`](./SCENARIOS.md) for the exact workload.

> **Numbers below are from one machine** (Apple M1 Max, macOS, 8 performance cores),
> recorded 2026-06-15 — indicative, not universal. Run it yourself.

## The three configurations

Biloba 0.3.0 ships two interaction tracks, so this is a **three-way** comparison, and
the **identical 32-scenario suite runs under all three**:

| config | how interactions run | speed |
|---|---|---|
| **biloba-fast** | the default `b` — fast, atomic JavaScript simulations (`el.click()`, value-set, synthetic events). No scroll-into-view, occlusion check, or real pointer. | fastest |
| **biloba-realistic** | `b.Realistic()` — the *same tab*, with interactions routed through real Chrome DevTools Protocol input (scroll-into-view, real pointer moves, occlusion-aware clicks, real keystrokes). | middle |
| **Playwright** | its only model — always realistic (real CDP input, actionability waits). | baseline |

biloba-fast and biloba-realistic are the *same* Ginkgo suite: every interaction goes
through one handle that is either `b` or `b.Realistic()` (selected by an env var), so
**only the interaction engine differs** between the two runs.

**The fast ↔ realistic tradeoff is a speed-for-fidelity dial.** Fast skips the
per-interaction CDP round-trips, so it's ~2× faster than realistic (serial) — use it
for the bulk of a suite. Realistic matches Playwright's input fidelity (real pointer,
real keys, scroll, occlusion) for the handful of tests that need it — and, as the
numbers show, still runs ~2× faster than Playwright whole-suite.

## Topline — whole-suite wall clock (8 workers, n=15)

Median wall clock ± stddev over 15 repeats:

| config | parallel(8) | serial(1) | vs Playwright (parallel / serial) |
|---|---:|---:|---|
| **biloba-fast** | **2.57 ± 0.06s** | **9.55 ± 0.05s** | **3.2× / 4.0×** |
| **biloba-realistic** | **3.26 ± 0.03s** | **18.60 ± 0.07s** | **2.5× / 2.1×** |
| **playwright** | 8.23 ± 0.24s | 38.37 ± 0.56s | — |

Each bar below splits the total into **fixed startup** (Biloba shares one Chrome; Playwright
launches one browser per worker) and **spec runtime**, with ±stddev error bars:

![whole-suite wall clock — startup vs spec runtime](charts/config.svg)

- biloba-fast is **3.2× faster parallel / 4.0× serial** than Playwright.
- biloba-realistic — doing the *same* real-CDP-input work Playwright does — is still
  **~2.5× faster parallel / ~2.1× serial**.
- Realism costs **1.27× parallel / 1.95× serial** over fast: the price of real input.

## By category — the same three configs, per bucket

Serial **mean per-test duration ± SEM** (~25 samples/scenario — the same numbers the chart
shows). This shows how the gap behaves as the work gets heavier. C/D have wide error bars
because those buckets mix fast and slow scenarios (latency / async); ratios use the means.

| bucket | biloba-fast | biloba-realistic | playwright | pw/fast | pw/real |
|---|---:|---:|---:|---:|---:|
| A/B static (reads) | 11.2 ± 0.2 ms | 25.7 ± 1.8 ms | 73.1 ± 1.7 ms | **6.5×** | 2.8× |
| C network | 83.6 ± 10.6 ms | 102.6 ± 10.5 ms | 249.9 ± 27.2 ms | 3.0× | 2.4× |
| D scale | 120.9 ± 17.2 ms | 175.5 ± 17.1 ms | 295.1 ± 32.8 ms | **2.4×** | 1.7× |
| F semantic locators | 15.7 ± 0.2 ms | 29.7 ± 1.2 ms | 69.8 ± 0.7 ms | 4.4× | 2.4× |
| G interaction vocabulary | 11.2 ± 0.1 ms | 47.0 ± 1.9 ms | 83.1 ± 1.1 ms | 7.4× | 1.8× |
| H pointer options | 11.2 ± 0.4 ms | 41.9 ± 1.2 ms | 71.9 ± 0.6 ms | 6.4× | 1.7× |
| E realism (occlusion/scroll) | 13.2 ± 0.3 ms | 163.2 ± 18.4 ms | 211.7 ± 19.1 ms | 16.0× | 1.3× |

![per-bucket per-test duration](charts/buckets.svg)

- biloba-fast's lead is **widest (~7×) on trivial DOM** — almost pure framework overhead —
  and **compresses to ~2.4×** once real browser work dominates (D scale), where both run on
  the same Chromium engine.
- biloba-realistic tracks **~2× faster than Playwright** wherever it does real input, and
  **converges toward it on the heaviest work** (D 1.7×) where both are engine-bound.
- **E realism** is the outlier: biloba-fast is near-instant there because its atomic click
  *skips* the actionability wait (occlusion) and scroll that a real user needs; realistic
  does that work, landing next to Playwright. (Use realistic — or `BeClickable()` — when that
  actionability matters.)

## Per-test — every scenario, all three configs

Each scenario plotted as Biloba per-test (y) vs its Playwright per-test (x), log–log, with the
`y=x` diagonal and 2×/4× reference lines; below the diagonal = Biloba faster. ~25 serial samples
per point, P25–P75 error whiskers (small — per-test timing is stable).

![per-test scatter — Biloba vs Playwright](charts/scatter.svg)

biloba-fast (green) sits near/below the **4× line** and is roughly flat — its cheap atomic path
barely moves with how much work Playwright is doing. biloba-realistic (blue) hugs the **2× line** —
it scales *with* Playwright (both CDP-input-bound) but stays about twice as fast.

## The workload

32 scenarios across 7 buckets, replicated `REPS` times (default 8 → 256 specs/config). The same
assertions run on every config. Buckets get heavier left-to-right:

| bucket | tests | what they exercise | rough DOM |
|---|---:|---|---|
| **A/B static** | 10 | read-only DOM (count/visibility/text/attr/class/prop) + basic interactions (click counter, form fill, real-key typing) | trivial (~15 nodes) |
| **C network** | 5 | observe a real fetch, stub it, wait through 300 ms latency, abort it, modify the real response (CDP `Fetch`/`Network`) | trivial (~6 nodes) |
| **D scale** | 4 | render a **1000-row** table, filter a large list by real keys, drive a gated 4-step wizard, await staggered async appends | **heavy (~2000 nodes)** |
| **E realism** | 2 | an occluding overlay (cleared at 250 ms) and a button ~4000 px below the fold | trivial (~5 nodes) |
| **F semantic locators** | 3 | select by role+name / visible text / form label against **~200 distractor** roled elements (exercises the accessible-name engine) | medium (~600 nodes) |
| **G interaction vocabulary** | 6 | double / right / middle-click, drag-and-drop, wheel scroll, tap | trivial (~12 nodes) |
| **H pointer options** | 2 | click at an offset (`At`) and modifier-click (`Shift`) | trivial (~4 nodes) |

The Biloba suite also carries 3 fast-only CSS-hook variants of Bucket F (to measure the
CSS-vs-locator cost inside Biloba — the locator engine adds ~1.1×); they're excluded from the
headline by label.

## Running it

Prereqs: Go, Node ≥18, a one-time browser install on each side (`biloba/README.md`,
`playwright/README.md`). Then:

```bash
./run.sh                 # topline table: all three configs, whole-suite wall clock
./scaling.sh             # startup vs runtime fit (behind the topline chart)
./pertest.sh             # per-test timing → the per-bucket table + scatter/bucket charts
REPS=16 ./buckets.sh     # cross-check: per-bucket marginal per-spec via wall clock (startup-subtracted)
```

The per-bucket table & charts come from `pertest.sh` (mean per-test); `buckets.sh` is an
independent wall-clock cross-check that lands in the same ballpark. The figures are SVGs under
[`charts/`](./charts), generated by the stdlib-only Go tool there.
Layout: `server/` (shared Go target + fixtures), `biloba/` (Ginkgo suite), `playwright/`
(`@playwright/test` suite), `charts/` (SVG generator), and the run scripts.
