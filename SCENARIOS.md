# Canonical scenario spec

This file is the **single source of truth** for the suites. Every base scenario
below must exist in *both* `biloba/` and `playwright/`, exercise the *same*
pathway, and make the *same* assertions. If you change a scenario, change it on
both sides.

## The three configurations

0.3.0 added a second interaction track, so the comparison is now **three-way** —
and the whole suite runs under all three:

- **Playwright** — the realism baseline. One interaction model, always realistic
  (CDP input, actionability waits, auto-scroll, occlusion checks).
- **biloba-fast** — the default `b`. Fast, atomic JavaScript simulations
  (`el.click()`, value-set, synthetic events). Biloba's recommended default and
  the **primary speed story**.
- **biloba-realistic** — `b.Realistic()`. The *same tab*, with interactions routed
  through real Chrome DevTools Protocol input — so it does the *same work*
  Playwright does. The **fair realism comparison**.

**The identical 32-scenario suite runs under each config.** biloba-fast and
biloba-realistic are the *same Ginkgo suite* — every interaction goes through one
handle `bi`, which is `b` for fast and `b.Realistic()` for realistic, selected by
the `BILOBA_REALISTIC` env var. Nothing else changes between the two runs: same
scenarios, same assertions, only the interaction engine swapped. This is the whole
point — "here is one test suite; what does it cost under each engine?"

### How the configs are selected in the Biloba suite

Per replication the Biloba suite has:

- **32 base specs** — the scenarios every config runs. Their count equals the
  Playwright spec count exactly. `BILOBA_REALISTIC=0` runs them through `b`
  (biloba-fast); `BILOBA_REALISTIC=1` runs them through `b.Realistic()`
  (biloba-realistic).
- **3 CSS-hook specs** — fast-only variants of the Bucket F locator scenarios that
  target a stable `#id` instead of a semantic locator (tagged `Label("csshook")`),
  used only to measure the **CSS-vs-locator** cost inside Biloba. They are excluded
  from every headline run (`--label-filter='!csshook'`).

So all three configs run the same 32 scenarios; `run.sh` produces a six-row
headline (3 configs × serial/parallel) and `buckets.sh` breaks each bucket down the
same three ways.

Every scenario must run identically under both Biloba handles, so two scenarios
were left out: a CSS-`:hover`-revealed menu (fast `Hover` doesn't fire CSS `:hover`)
and clicking into an open shadow root under realistic mode (a known
realistic-mode limitation — its occlusion check can't pierce shadow retargeting).

## Ground rules (apply to every scenario)

- **Target:** the shared server at `BASE_URL` (default `http://127.0.0.1:9889`),
  read from the environment. Never hard-code a port.
- **Readiness anchor:** after navigating, wait for `#heading` to be present before
  doing anything else. This is the page-ready gate on every side.
- **Auto-waiting assertions everywhere.** Use each framework's idiomatic *polling*
  assertion for *all* checks — Biloba `Eventually(...).Should(...)`, Playwright
  web-first `await expect(...)...`. Do **not** use a non-waiting immediate read on
  one side and an auto-waiting assertion on the other; that would bias the timing.
- **Aligned max-wait timeouts.** Both sides cap auto-waiting at **5s** (Playwright's
  default `expect` timeout; the Biloba suite sets `SetDefaultEventuallyTimeout(5s)`
  to match — Gomega's default is otherwise 1s). This governs only robustness under
  load, not happy-path timing (both return as soon as the condition holds), so it
  keeps neither side more patient than the other.
- **Same assertion count per scenario.** The bullet list under each scenario is
  exhaustive — assert exactly those things, no more, no less. Every config (fast,
  realistic, Playwright) asserts the **same observable outcome**.
- **Replication factor `REPS`.** Read integer env `REPS` (default `8`). Wrap the
  whole scenario set in a table/loop that runs it `REPS` times with labels
  `rep-1 … rep-REPS`. Base specs = `32 × REPS` (identical across all three configs);
  the Biloba suite additionally carries `3 × REPS` fast-only CSS-hook specs. This
  exists only to create enough independent, parallelizable work to measure.
- **Bucket labels.** Keep the `Describe`/`describe` titles starting with the page
  or bucket name shown below (e.g. "Bucket D — scale", "Bucket E — realism") so the
  harness can time each bucket separately by title filter.
- **Isolation:** each spec is independent and must pass in any order and in parallel.

## Browser engine

Both suites must run **headless** so both drive `chrome-headless-shell` /
`chromium-headless-shell`. Do not run headed. Do not enable video/trace/artifacts
(they are pure overhead and asymmetric). No retries.

---

> Every bucket below runs under all three configs (biloba-fast = `bi` is `b`,
> biloba-realistic = `bi` is `b.Realistic()`, Playwright). The interaction handle in
> the Biloba suite is always `bi`; reads/assertions are identical across configs.

## Page A — `dom.html` (read-only DOM)

Navigate to `BASE_URL + "/dom.html"`, wait for `#heading`.

- **A1 — navigation & title** — title is `DOM Fixture`; `#heading` text is `Widgets`
- **A2 — count** — `.item` count is `4`
- **A3 — visibility** — `.item:not(.hidden)` count is `3`; `.item.hidden` is not visible
- **A4 — inner text of all matches** — inner text of every `.item` is exactly `["Alpha","Bravo","Charlie","Delta"]`
- **A5 — attribute** — `#status` attribute `data-state` is `ready`
- **A6 — class** — `#status` has class `muted`
- **A7 — property** — `#docs-link` `href` property is `https://example.com/docs`

## Page B — `interactions.html`

Navigate to `BASE_URL + "/interactions.html"`, wait for `#heading`.

- **B1 — click (counter)** — click `#increment` three times; `#count` text is `3`
- **B2 — form fill (value-set semantics, fires input/change, NOT real keys)**
  - set `#name` to `Jane` (Biloba `SetValue` / Playwright `fill`); select `#role`
    option `editor`; check `#subscribe`; click `#save`; `#result` text is
    `Jane / editor / subscribed`
- **B3 — real keystroke typing (search-as-you-type, needs genuine `keyup`)**
  - type `ap` into `#search` with **real keys** (Biloba `Type` / Playwright
    `pressSequentially`) — *not* `SetValue`/`fill`
  - visible `.fruit` count is `2`; inner text of the visible `.fruit`s is exactly
    `["Apple","Apricot"]`

## Page C — `network.html`

Navigate to `BASE_URL + "/network.html"`, wait for `#heading`. Network interception
is CDP-level on both frameworks; the realistic track behaves like fast here (only
the `#load` click differs).

- **C1 — observe a real request** — click `#load`; `.result` count is `3`; inner
  text of every `.result` is exactly `["One","Two","Three"]`
- **C2 — stub the request (short-circuit, no real round-trip)**
  - stub/route `GET /api/items` → `{"items":["Stubbed"]}` (Biloba `StubRequest` /
    Playwright `page.route` + `route.fulfill`); the request never reaches the server
  - click `#load`; `.result` count is `1`; inner text is exactly `["Stubbed"]`
- **C3 — wait through latency** — click `#load-slow` (fixed 300ms delay); `.result`
  count is eventually `3`
- **C4 — abort the request** *(new in 0.3.0)*
  - abort `GET /api/items` (Biloba `AbortRequest` / Playwright `route.abort()`);
    the page's `fetch` rejects and renders a single `.result` = `Error`
  - click `#load`; `.result` count is `1`; inner text is exactly `["Error"]`
- **C5 — modify the real response** *(new in 0.3.0)*
  - let `GET /api/items` hit the **real** server, then rewrite the response body to
    `{"items":["Modified"]}` (Biloba `ModifyResponse(...).WithBody(...)` /
    Playwright `route.fetch()` then `route.fulfill({response, body})`). Both pay a
    real round-trip (unlike C2's short-circuit stub).
  - click `#load`; `.result` count is `1`; inner text is exactly `["Modified"]`

## Bucket D — speed-at-scale (apples-to-apples)

Real DOM weight / multi-step flows. All configs behave identically and pass the
same assertions; this asks whether the framework-overhead gap holds once there is
real browser work to do.

- **D1 — large table render** (`scale.html`) — `.row` count is `1000`; the `.name`
  cell of `.row[data-id='500']` has text `Item 500`
- **D2 — filter a large list with real keys** (`scale.html`) — type `cat-3` into
  `#q` with **real keys**; visible `.row` count is `200`
- **D3 — gated multi-step wizard** (`wizard.html`) — each Next is disabled until its
  field is filled; set `#input1`=`a` → `#next1` → … → `#finish4`; `#summary` is `a-b-c-d`
- **D4 — staggered async** (`async.html`) — click `#start` (appends 10 `.async-item`,
  one every 40ms); `.async-item` count is eventually `10`; last is `Item 10`

## Bucket E — occlusion & scroll

Two interactions where biloba-fast's atomic click is near-instant because it skips work
the others do. Identical code under both handles: under `b` it skips, under `b.Realistic()`
it does the work Playwright does (so realistic ≈ Playwright here).

- **E1 — occlusion** (`occlusion.html`) — full-screen overlay covers the button, removed at 250ms
  - click `#occluded-btn`; `#occ-count` text is `1`
  - fast clicks through the overlay at ~0ms; realistic and Playwright wait until it clears (~250ms).
- **E2 — scroll-into-view** (`scroll.html`) — button ~4000px below the fold
  - click `#below-btn`; `#scroll-result` is `clicked`
  - fast clicks without scrolling; realistic and Playwright scroll the button into view first.

## Bucket F — semantic locators (+ CSS-hook sub-comparison)

Navigate to `BASE_URL + "/locators.html"`, wait for `#heading`. The page has ~200
distractor roled/named elements so the accessible-name engines (Biloba's
`ByRole`/`ByText` and Playwright's `getByRole`/`getByText`) have a real
full-document scan to do — the **slowest selection pathway** per the skills. This
bucket measures that engine cost as a near-1:1 analog.

- **F1 — role + name** — Biloba `b.ByRole("button").WithName("Save")` / Playwright
  `getByRole('button', {name:'Save', exact:true})`: click it; `#save-result` is `saved`
- **F2 — visible text** — Biloba `b.ByText("Featured item")` / Playwright
  `getByText('Featured item', {exact:true})`: it is visible
- **F3 — form label** — Biloba `b.ByLabel("Email")` / Playwright `getByLabel('Email')`:
  set it to `x@y.com` (value-set); its value is `x@y.com`

**CSS-hook sub-comparison (Biloba only, fast, `Label("csshook")`).** The same three
scenarios, but targeting the stable `#id` (`#save-btn`/`#featured`/`#email`) instead
of a locator. Same assertions. This quantifies what the locator convenience costs vs
Biloba's recommended CSS default. (Excluded from every headline run by label.)

## Bucket G — interaction vocabulary

Navigate to `BASE_URL + "/vocab.html"`, wait for `#heading`. Each is a near-1:1
analog. Under fast `bi`=`b` dispatches synthetic events; under realistic
`bi`=`b.Realistic()` uses real CDP input; Playwright uses its native equivalent.
All assert the same sentinel text.

- **G1 — double-click** — Biloba `DblClick("#dbl-btn")` / Playwright `dblclick()`;
  `#dbl-result` is `double`
- **G2 — right-click** — Biloba `RightClick("#ctx-btn")` / Playwright
  `click({button:'right'})`; `#ctx-result` is `menu`
- **G3 — middle-click** — Biloba `MiddleClick("#aux-btn")` / Playwright
  `click({button:'middle'})`; `#aux-result` is `middle`
- **G4 — drag-and-drop** — Biloba `DragTo("#drag-src","#drop-zone")` / Playwright
  `dragTo()` (pointer-based DnD); `#drop-result` is `dropped`
- **G5 — wheel scroll** — Biloba `ScrollWheel("#scroll-box", 0, 200)` / Playwright
  `hover()` then `mouse.wheel(0, 200)`; `#wheel-result` is `wheeled`
- **G6 — tap (touch)** — Biloba `Tap("#tap-btn")` / Playwright `tap()` (Playwright
  needs `hasTouch:true` context; Biloba does not); `#tap-result` is `tapped`

## Bucket H — pointer options

Navigate to `BASE_URL + "/pointer.html"`, wait for `#heading`.

- **H1 — click at offset** — Biloba `Click("#click-pad", bi.At(30,40))` / Playwright
  `click({position:{x:30,y:40}})`; `#click-pad-result` is `ok` (the fixture records
  `ok` when the click landed within ±2px of the (30,40) offset — a default centre
  click would be ~(100,100), so `ok` still proves the offset took effect)
- **H2 — modifier-click** — Biloba `Click("#mod-btn", b.Shift())` / Playwright
  `click({modifiers:['Shift']})`; `#mod-result` is `shift`

---

## Equivalence notes (read before implementing)

- **One suite, three configs, identical outcomes.** Every scenario uses the handle
  `bi` for interactions; biloba-fast runs with `bi`=`b`, biloba-realistic with
  `bi`=`b.Realistic()`, and Playwright asserts the same observable result. Only the
  interaction engine differs between the two Biloba runs.
- **B2 vs B3 is deliberate.** B2 uses each framework's *value-set* path (events, no
  real keys); B3 forces *real keystrokes* through CDP input. Keeping both avoids
  cherry-picking.
- **C2 vs C5 is deliberate.** C2 short-circuits (the request never reaches the
  server); C5 modifies the *real* response (a real round-trip on both sides). C4 is
  a true abort (the page's fetch rejects).
- **C1/C3 are latency-bound** (a fixed 300ms server delay on C3) and *mostly* but
  not *perfectly* framework-neutral — each framework observes the settled result on
  its own polling cadence (Gomega `Eventually` ~10ms; Playwright web-first `expect`
  ~100/250/500ms). We keep both at their defaults and disclose this. See README
  "Threats to validity".
- **Counts and texts are exact.** "inner text of every X is exactly [...]" means
  collect all matches in document order and compare the full slice/array.
- **Selectors carry the same meaning on both sides.** CSS strings are
  character-for-character identical; F1–F3 select by the same role/text/label.
