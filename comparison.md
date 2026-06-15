# Biloba vs Playwright — measured results

From **one machine** (Apple M1 Max, macOS 26.5.1; parallel runs pinned to its 8 performance
cores via `WORKERS=8`), recorded 2026-06-15 — indicative, not universal. Re-run `./run.sh`,
`./scaling.sh`, `./buckets.sh`, `./pertest.sh` on your own hardware. Methodology and
fairness controls: [`METHODOLOGY.md`](./METHODOLOGY.md). Workload: [`SCENARIOS.md`](./SCENARIOS.md).

- **Three configs, one suite:** the identical 32 scenarios run under **biloba-fast** (`b`),
  **biloba-realistic** (`b.Realistic()`), and **Playwright** — only the interaction engine
  differs between the two Biloba runs. `REPS=8` → 256 specs/config, same assertions throughout.
- **Engine:** all launch `chrome-headless-shell` — Playwright 148.0.7778.96, Biloba 149.0.7827.115.
- **Discipline:** warm (one discarded warmup); no retries/video/trace/screenshots; configs run
  sequentially and interleaved (never concurrently), rotating the leader; auto-wait timeout
  aligned at 5s. Stress-run 15× parallel per Biloba config (+5× Playwright) with zero flakes.

## 1. Whole-suite wall clock (8 workers, n=15)

| config | median | mean | stddev | min | max |
|---|---:|---:|---:|---:|---:|
| biloba-fast parallel(8) | **2.57** | 2.59 | 0.06 | 2.54 | 2.80 |
| biloba-realistic parallel(8) | **3.26** | 3.25 | 0.03 | 3.20 | 3.29 |
| playwright parallel(8) | **8.23** | 8.34 | 0.24 | 8.14 | 8.88 |
| biloba-fast serial(1) | **9.55** | 9.57 | 0.05 | 9.51 | 9.71 |
| biloba-realistic serial(1) | **18.60** | 18.62 | 0.07 | 18.52 | 18.78 |
| playwright serial(1) | **38.37** | 38.63 | 0.56 | 38.16 | 40.11 |

- **biloba-fast vs Playwright:** 3.2× faster parallel, 4.0× serial.
- **biloba-realistic vs Playwright:** 2.5× faster parallel, 2.1× serial (same real-CDP-input work).
- **Realism cost (realistic vs fast):** 1.27× parallel, 1.95× serial.

Each bar split into fixed startup + spec runtime (from §4's fit), ±stddev over the 15 repeats:

![whole-suite wall clock — startup vs spec runtime](charts/config.svg)

## 2. By category — marginal per-spec (serial)

Each config's per-invocation startup is fit and subtracted, so a small focused bucket isn't
startup-dominated. Worker-independent; shows how the gap behaves as the work gets heavier.

| bucket | biloba-fast | biloba-realistic | playwright | pw/fast | pw/real | what it measures |
|---|---:|---:|---:|---:|---:|---|
| A/B static (reads) | 10.3 ms | 24.4 ms | 80.0 ms | **7.8×** | 3.3× | framework overhead, trivial DOM |
| C network | 86.1 ms | 106.9 ms | 259.8 ms | 3.0× | 2.4× | CDP interception + latency |
| D scale | 124.0 ms | 180.4 ms | 306.3 ms | **2.5×** | 1.7× | real DOM weight (1000-row, wizard, async) |
| F semantic locators | 14.4 ms | 29.5 ms | 81.2 ms | 5.6× | 2.8× | ARIA-locator engine (see §3) |
| G interaction vocabulary | 10.2 ms | 47.4 ms | 90.7 ms | 8.9× | 1.9× | dbl/right/middle/drag/wheel/tap |
| H pointer options | 10.0 ms | 42.2 ms | 82.0 ms | 8.2× | 1.9× | offset + modifier click |
| E realism | 11.1 ms | 165.4 ms | 235.2 ms | 21.2× | 1.4× | occlusion + scroll |

![per-bucket per-test duration](charts/buckets.svg)

- The fast lead is **widest (~8×) on trivial DOM** (near-pure framework overhead) and
  **compresses to ~2.5×** once real browser work dominates (D), where both run on the same engine.
- **biloba-realistic tracks ~2× faster than Playwright** wherever it does real input, converging
  on the heaviest work (D 1.7×).
- **E realism** is the outlier: biloba-fast's atomic click is near-instant because it *skips* the
  occlusion wait (~250 ms) and the 4000 px scroll; realistic does that actionability work and lands
  next to Playwright. Per scenario (marginal per-spec):

  | scenario | biloba-fast | biloba-realistic | playwright |
  |---|---:|---:|---:|
  | E1 occlusion | 10.7 ms | 298.0 ms | 391.6 ms |
  | E2 scroll-into-view | 10.9 ms | 33.7 ms | 84.3 ms |

  On E1 both realistic and Playwright wait for the overlay; realistic is faster mostly because
  Gomega `Eventually` polls ~10 ms vs Playwright's `[100,250,500,1000]ms` cadence (it catches the
  250 ms clear sooner). Reach for realistic mode — or the atomic `BeClickable()` guard — when that
  actionability matters.

## 3. Per-test scatter & the locator cost

![per-test scatter — Biloba vs Playwright](charts/scatter.svg)

Each scenario plotted Biloba (y) vs Playwright (x), log–log, ~25 serial samples/point, P25–P75
whiskers. biloba-fast (green) sits near/below the **4× line** and is roughly flat; biloba-realistic
(blue) hugs the **2× line**, scaling with Playwright but ~2× faster.

**CSS-vs-locator (Bucket F, fast):** selecting by semantic locator vs a CSS `#id` against ~200
distractor roled elements costs **1.1×** inside Biloba (14.4 vs 12.7 ms) — the ARIA scan is nearly
free. Playwright's `getByRole`/`getByText`/`getByLabel` is **6.4×** Biloba's CSS hook.

## 4. Startup vs runtime — least-squares fit `time = startup + perSpec × specs`

Running the identical workload at several sizes and fitting separates fixed startup (intercept)
from marginal per-spec runtime (slope). 8 workers, 32 base scenarios. This feeds §1's chart.

| config | startup | per-spec | runtime @256 | total @256 | R² |
|---|---:|---:|---:|---:|---:|
| biloba-fast parallel(8) | **0.78s** | **7.1 ms** | 1.81s | 2.59s | 0.9999 |
| biloba-realistic parallel(8) | 0.71s | 10.0 ms | 2.56s | 3.26s | 1.0000 |
| playwright parallel(8) | **1.98s** | **24.5 ms** | 6.26s | 8.24s | 0.9998 |
| biloba-fast serial(1) | 0.25s | 36.7 ms | 9.39s | 9.64s | 1.0000 |
| biloba-realistic serial(1) | 0.26s | 71.6 ms | 18.33s | 18.59s | 1.0000 |
| playwright serial(1) | 1.10s | 145.9 ms | 37.34s | 38.44s | 1.0000 |

The fitted `total@256` matches the §1 headline within noise (e.g. parallel 2.59 vs 2.57, 8.24 vs
8.23), cross-validating the model.

- **Startup is ~24% of Playwright's parallel wall (1.98s)** — it launches one browser per worker (8
  browsers); Biloba (fast *and* realistic) shares **one Chrome** (~0.75s).
- **Excluding startup, Playwright's marginal per-spec is 3.4× biloba-fast** parallel and **4.0× serial**;
  biloba-realistic sits between (real CDP input costs more per spec than the atomic default, but stays
  well under Playwright).

## Bottom line

- biloba-fast is **~3.2× parallel / ~4.0× serial** faster than Playwright whole-suite — widest (~8×)
  on light DOM, compressing to ~2.5× once real browser work dominates. ~24% of Playwright's parallel
  wall is one-browser-per-worker startup; the rest is ~3–4× higher marginal per-spec cost.
- biloba-realistic, doing the same real-CDP-input work, is **~2.5× parallel / ~2.1× serial** faster
  than Playwright — the realistic track costs ~1.95× over fast (serial), the price of real input.
- Semantic locators cost Biloba ~1.1× over its CSS default.
