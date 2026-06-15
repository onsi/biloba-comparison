# Biloba comparison suite

The Biloba (Go / Ginkgo / Gomega) side of the comparison. Implements the canonical
[`../SCENARIOS.md`](../SCENARIOS.md). It is its own Go module that depends on the
released `github.com/onsi/biloba` (v0.3.0).

## One-time setup

```bash
go mod tidy
# Biloba auto-installs chrome-headless-shell on first run via
# biloba.AutoInstallHeadlessShell(); or install it yourself:
npx @puppeteer/browsers install chrome-headless-shell@stable
```

## Run it directly

```bash
# point at the shared server (see ../server) and choose a replication factor
BASE_URL=http://127.0.0.1:9889 REPS=8 ginkgo -p .     # parallel (all 35 specs/rep)
BASE_URL=http://127.0.0.1:9889 REPS=8 ginkgo .        # serial

# precompiled (what the harness times):
ginkgo build .
BASE_URL=... ginkgo --procs=10 ./biloba.test

# the two Biloba configurations are the SAME suite, env-selected:
BILOBA_REALISTIC=0 ginkgo --label-filter='!csshook' ./biloba.test   # biloba-fast      (bi = b)
BILOBA_REALISTIC=1 ginkgo --label-filter='!csshook' ./biloba.test   # biloba-realistic (bi = b.Realistic())
ginkgo --label-filter='csshook' ./biloba.test                       # fast-only CSS-hook F variants (3/rep)
```

This suite carries **35 specs/rep**: the **32 base** scenarios (every interaction
goes through `bi`, which is `b` under `BILOBA_REALISTIC=0` and `b.Realistic()` under
`BILOBA_REALISTIC=1` — the *same* scenarios run both ways) plus **3 fast-only
CSS-hook** Bucket-F variants (`Label("csshook")`, used for the CSS-vs-locator
table). See `../SCENARIOS.md`. Config: `SpinUpChrome(AutoInstallHeadlessShell())`,
default headless shell, shared browser + reused root tab per process (Biloba's
recommended best practice). All assertions auto-wait via `Eventually`.
