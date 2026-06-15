# Methodology & fairness

The fairness controls and threats-to-validity behind the numbers in
[`README.md`](./README.md) / [`comparison.md`](./comparison.md). This is a deliberately
**fair, reproducible** comparison between [Biloba](https://github.com/onsi/biloba) (Go, on
chromedp) and [Playwright Test](https://playwright.dev) (TypeScript/Node — the fastest,
reference Playwright binding). The aim is *not* to make Biloba win — it is to measure
framework + runtime overhead for an identical browser-test workload under each framework's
own recommended configuration, and to report the numbers honestly with their caveats.

> **Author's stance.** Biloba and this repo are written by the same person. That is
> exactly why this harness is structured to be auditable by a skeptic: one shared
> server, one canonical scenario list implemented identically on both sides, a
> framework-neutral external stopwatch, and an explicit list of the biases we could
> *not* design away. Read [`SCENARIOS.md`](./SCENARIOS.md) and audit both suites.

## Layout

```
README.md           high-level overview + takeaways
comparison.md       the full measured results
SCENARIOS.md        canonical, framework-neutral spec — every config implements this
server/             the single shared target (Go, stdlib only, embedded fixtures)
biloba/             the Biloba suite (Go / Ginkgo / Gomega)
playwright/         the Playwright suite (TypeScript / @playwright/test)
run.sh              headline harness (whole-suite wall clock, all three configs)
scaling.sh          separates fixed startup from marginal per-spec runtime
buckets.sh          three-way per-bucket timing (fast / realistic / Playwright)
pertest.sh          captures per-test timing + renders the SVG charts
charts/             stdlib-only Go SVG generator (config / buckets / scatter)
```

## Three configurations (new in 0.3.0)

0.3.0 shipped a second interaction track, so the comparison is now **three-way** —
and **the identical 32-scenario suite runs under all three**:

- **Playwright** — the realism baseline; one interaction model, always realistic.
- **biloba-fast** — the default `b`; fast atomic JS simulations. Biloba's
  recommended default and the **primary speed story**.
- **biloba-realistic** — `b.Realistic()`; the same tab with interactions routed
  through real CDP input, so it does the *same work* Playwright does. The **fair
  realism comparison**.

biloba-fast and biloba-realistic are the *same Ginkgo suite*: every interaction goes
through one handle `bi`, which is `b` (`BILOBA_REALISTIC=0`) or `b.Realistic()`
(`BILOBA_REALISTIC=1`). So `run.sh` produces a **six-row headline** (3 configs ×
serial/parallel) and `buckets.sh` breaks every bucket down the same three ways.

## Scenario buckets

The 32 base scenarios (`SCENARIOS.md`) are grouped so the harness can time them
separately and you can see how the gap behaves as the work gets heavier — each
bucket runs under all three configs:

- **A/B (10)** — read-only DOM + simple interactions. Trivial DOM; pure framework
  overhead per operation.
- **C (5)** — network: observe / stub / latency / **abort** / **modify-response**.
- **D (4)** — *speed-at-scale*: 1000-row table, large-list filter, gated wizard,
  staggered async. Asks whether the gap holds as real browser work enters.
- **E (2)** — occlusion, scroll-into-view: biloba-fast's atomic click is near-instant
  because it *skips* the actionability wait / scroll the others do, so realistic ≈ Playwright here.
- **F (3)** — *semantic locators*: role+name / text / label — a near-1:1 analog of
  Playwright's `getByRole`/`getByText`/`getByLabel`, measuring the ARIA-locator
  engine. Plus a Biloba-internal **CSS-hook vs locator** sub-comparison.
- **G (6)** — *interaction vocabulary*: double/right/middle-click, drag, wheel, tap.
- **H (2)** — *pointer options*: click-at-offset (`b.At`) and modifier-click.

Two scenarios were left out because they can't run identically under both Biloba handles:
a CSS-`:hover`-revealed menu (fast `Hover` doesn't fire CSS `:hover`) and clicking into an
open shadow root under realistic mode (a known realistic-mode limitation). See `SCENARIOS.md`.

## What is held identical (so the comparison is apples-to-apples)

1. **One shared server, identical fixtures.** Both suites navigate to the *same*
   running process at `BASE_URL`. The server is stdlib-only and deterministic (the
   "slow" endpoint sleeps a fixed 300ms). It therefore cancels out of the timing.
2. **One canonical scenario list.** [`SCENARIOS.md`](./SCENARIOS.md) fixes every
   navigation, action, selector, and assertion. All three configs run the **same
   `32 × REPS` specs** making the **same number of assertions**; `run.sh` asserts
   this for all three (fatal on mismatch). The Biloba suite additionally carries
   `3 × REPS` fast-only CSS-hook specs, filtered out of every headline by label.
3. **Same browser engine class.** Both run **headless**, so both drive
   `chrome-headless-shell` (Biloba) / `chromium-headless-shell` (Playwright ≥1.43
   auto-selects it for headless). `run.sh` prints both browser versions so you can
   see they match in class. No video, no trace, no screenshots, no retries on
   either side — those are asymmetric overhead.
4. **Same parallelism degree.** `WORKERS` (default = CPU count) is applied to both:
   `ginkgo -p -procs=$WORKERS` and Playwright `--workers=$WORKERS`. Serial runs pin
   both to 1.
5. **Same workload shape.** `REPS` replicates the scenario set identically on both
   sides to create enough independent, parallelizable work to measure.
6. **Neutral stopwatch.** The headline number is external wall-clock (via
   `Time::HiRes`), measured around each framework's own runner — not self-reported
   by either framework, and not instrumented in a way that touches only one side.

## What is intentionally NOT identical (and why that's the fair choice)

Each framework runs at **its own recommended best practice**, because forcing one
to mimic the other would itself be unfair. The two legitimate differences:

- **Browser topology.** Biloba runs **one shared Chrome** process; each Ginkgo
  parallel process drives its own isolated tab (BrowserContext) on it. Playwright's
  recommended model launches **one browser per worker**, reused across that worker's
  tests, with a fresh BrowserContext+page per test. This difference is *core to what
  is being measured* — it is a real architectural choice, not a thumb on the scale.
- **Isolation unit.** Biloba reuses one root tab per process and resets it with
  `Prepare()`; Playwright creates a fresh context per test. Both deliver per-test
  isolation via incognito-like contexts; they differ in whether the context object
  is reused or recreated.

## Threats to validity we could NOT fully neutralize (be skeptical here)

- **Language/runtime.** Go (compiled) vs Node (JIT + per-run TS transform). We
  exclude one-time compilation/transform from the timed region (Go test binary is
  prebuilt; a warmup run primes Playwright's transform cache and both warmups are
  discarded), but steady-state runtime differences remain and are part of the result.
- **Browser topology, again.** One-shared-browser vs browser-per-worker changes
  memory and startup amortization. We report both **whole-suite wall clock**
  (includes startup) and per-config medians so you can see startup's share.
- **Polling cadence is a measurable, non-trivial edge to Biloba — quantified here.**
  Both use auto-waiting assertions, but the default cadences differ: Gomega
  `Eventually` polls ~every 10ms; Playwright's web-first `expect` retries on a
  hard-coded `[100,250,500,1000]ms` schedule (`playwright-core` `coreBundle.js`),
  and that interval is **not** user-configurable for web-first matchers (only the
  `timeout` is — you must drop to `expect.poll`/`toPass`, which *do* take
  `intervals`, to change it). On the *settled* happy path both succeed on the first
  poll, so this is a wash for the 10 synchronous A/B specs. But on the 3 network
  specs (C1/C2/C3) Playwright waits for its next poll tick. Measured directly
  (tightening Playwright's network polling to 10ms via `expect.poll`): the cadence
  costs Playwright **~168ms/spec on the network specs, ~4s total, ≈30% of the whole
  serial gap** — all of it concentrated in those 3 specs. The remaining ~70% is
  per-test overhead: on the static A/B subset, where cadence cannot apply,
  Playwright is still ~6.9× slower per spec, and even with cadence fully neutralized
  it stays ~3× slower serially. We keep both at their library defaults (each is the
  idiomatic setting) and disclose the split rather than hand-tune either side.
- **Launch flags & viewport are each framework's defaults**, not normalized. For
  this light DOM/fetch workload (no paint/scroll/GC pressure) this is immaterial,
  but it is undisclosed-by-default and noted here for completeness.
- **Browser version skew.** Biloba auto-installs a `chrome-headless-shell` whose
  version is resolved at runtime; Playwright pins its own bundled
  `chromium-headless-shell`. They are usually within a version or two (e.g. 148 vs
  149). `run.sh` prints both launched binaries and versions so you can see the gap.
- **Single machine, single OS.** Numbers are indicative of *this* machine. The
  harness interleaves the six configurations (3 configs × serial/parallel) across repetitions to average out
  thermal drift (and alternates which framework leads each rep), discards a warmup,
  and reports median/mean/stddev/min/max — but absolute numbers vary by hardware.
- **Author bias.** Acknowledged above. Mitigation: everything is in-repo and
  auditable; both suites follow their framework's published skills/best-practices,
  and an adversarial fairness audit of both suites is part of how this was built.

## Running it

Prereqs: Go, Node ≥18, and a one-time browser install on each side (see
`biloba/README.md` and `playwright/README.md`). Then:

```bash
cd comparison
./run.sh                 # headline: biloba-fast vs Playwright, whole-suite wall clock
WORKERS=4 REPS=16 K=7 ./run.sh
./scaling.sh             # separates fixed startup from marginal per-spec runtime
REPS=16 ./buckets.sh     # three-way per-bucket: fast / realistic / Playwright, marginal per-spec
./pertest.sh             # per-test timing (N≈25 serial) → charts/{scatter,buckets}.svg
```

The figures in `comparison.md` are SVGs under `charts/`, generated by the stdlib-only
Go tool there: `pertest.sh` renders the per-test scatter and per-bucket bars; the
startup-vs-runtime `config.svg` is rendered from `charts/summary.json` (transcribed
from `run.sh` + `scaling.sh`) via `cd charts && go run . config summary.json .`.

`run.sh` is the **top-line** whole-suite view: all three configs, serial and
parallel (six rows). `buckets.sh` is the **by-category breakdown** of those same
three configs, subtracting each framework's measured startup intercept to report
*marginal* per-spec cost (so small focused buckets aren't startup-dominated).

`run.sh` starts the shared server, verifies all three configs report the **same spec
count** (fatal mismatch), runs a discarded warmup, then times serial and parallel
runs of each config — sequentially, never concurrently, rotating which config leads
each rep — across `K` repetitions, and prints median / mean / stddev / min / max.

`scaling.sh` answers **"how much of the wall clock is startup vs runtime?"** It runs
the identical workload at several sizes (`REPS_LIST`) and least-squares fits
`time = startup + perSpec × specs` for each of the six configs: the **intercept is
fixed startup** (process + browser launch + suite setup/teardown), the **slope is
the marginal per-spec runtime**, and `R²` validates the linear model. Applied
identically to every config, so it's neutral.

Results are not committed — run it on your own hardware.
