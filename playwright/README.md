# Playwright suite

The Playwright side of the Biloba-vs-Playwright comparison. It implements the
canonical scenario spec in [`../SCENARIOS.md`](../SCENARIOS.md) as an idiomatic
`@playwright/test` (TypeScript) suite, using web-first auto-waiting assertions
throughout.

- **Playwright version:** `@playwright/test` **1.61.0** (Chromium headless shell).
- **Engine:** headless — Playwright auto-selects `chrome-headless-shell` from the
  `chromium_headless_shell` bundle. No video, trace, screenshots, or retries.

## Install (one-time)

```bash
cd comparison/playwright
npm install                       # installs @playwright/test (dev dependency)
npx playwright install chromium   # installs chromium + chromium-headless-shell
```

## Run

The suite targets the shared server at `BASE_URL` (no hard-coded port) and
replicates the base scenario set `REPS` times (default `8`) — total tests =
`32 * REPS`. Workers come from the CLI; nothing is hard-pinned.

Playwright has a single, always-realistic interaction model, so it implements the
**base** set once. The Biloba suite runs the same 32 scenarios twice — through `b`
(biloba-fast) and `b.Realistic()` (biloba-realistic) — plus a fast-only CSS-hook
variant of Bucket F (see `../SCENARIOS.md`).

```bash
# from comparison/server, in another shell: PORT=9889 go run .
BASE_URL=http://127.0.0.1:9889 REPS=8 npx playwright test --workers=4
BASE_URL=http://127.0.0.1:9889 REPS=8 npx playwright test --list   # 256 tests
```

## Layout

- `playwright.config.ts` — `testDir: ./tests`, `fullyParallel: true`,
  `retries: 0`, headless, `line` reporter, all artifacts off, `baseURL` from
  `process.env.BASE_URL`.
- `tests/scenarios.spec.ts` — the 32 base scenarios (A/B static, C network incl.
  abort/modify, D scale, E realism, F semantic locators, G interaction vocabulary,
  H pointer options), wrapped in a `REPS` loop with `rep-1 … rep-REPS` describe blocks.
