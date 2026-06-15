#!/usr/bin/env bash
# Captures PER-TEST timing for all three configs and renders the SVG charts
# (charts/scatter.svg, charts/buckets.svg). Runs each config SERIALLY (no parallel
# contention) and aggregates each scenario's median across REPS samples.
#
# This is illustrative tooling on top of the headline (run.sh) and per-bucket
# (buckets.sh) numbers — it does not feed them. Reporters emit machine-readable
# per-spec durations (Ginkgo --json-report; Playwright --reporter=json); the
# stdlib-only Go tool in charts/ turns them into SVGs.
set -euo pipefail
cd "$(dirname "$0")"

REPS="${REPS:-25}"   # samples per scenario (each rep = one independent instance) → point + error bars
PORT="${PORT:-9889}"
export BASE_URL="${BASE_URL:-http://127.0.0.1:$PORT}"
export REPS
FAST_JSON=/tmp/cmp-report-fast.json
REAL_JSON=/tmp/cmp-report-real.json
PW_JSON=/tmp/cmp-report-pw.json

echo "REPS=$REPS (serial)  BASE_URL=$BASE_URL"
echo "==> building + starting shared server"
( cd server && go build -o /tmp/cmp-server . )
PORT="$PORT" /tmp/cmp-server >/tmp/cmp-server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf "$BASE_URL/dom.html" >/dev/null 2>&1 && break; sleep 0.1; done

echo "==> precompiling Biloba suite + charts tool"
( cd biloba && ginkgo build . >/dev/null )
BIN="$PWD/biloba/biloba.test"
( cd charts && go build -o /tmp/cmp-charts . )

echo "==> capturing per-test timing (serial)"
BILOBA_REALISTIC=0 ginkgo --procs=1 --label-filter='!csshook' --json-report="$FAST_JSON" "$BIN" >/dev/null 2>&1
echo "    biloba-fast      -> $FAST_JSON"
BILOBA_REALISTIC=1 ginkgo --procs=1 --label-filter='!csshook' --json-report="$REAL_JSON" "$BIN" >/dev/null 2>&1
echo "    biloba-realistic -> $REAL_JSON"
( cd playwright && npx playwright test --workers=1 --reporter=json >"$PW_JSON" 2>/dev/null )
echo "    playwright       -> $PW_JSON"

echo "==> rendering charts"
/tmp/cmp-charts pertest "$FAST_JSON" "$REAL_JSON" "$PW_JSON" charts
