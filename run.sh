#!/usr/bin/env bash
# Framework-neutral measurement harness for the three-way comparison:
# biloba-fast (b) vs biloba-realistic (b.Realistic()) vs Playwright.
#
# It starts the single shared server, confirms all three configs report the SAME
# spec count, discards a warmup, then times serial + parallel runs of each config
# interleaved across K repetitions and prints median/mean/stddev/min/max — SIX rows
# (3 configs x serial/parallel). biloba-fast and biloba-realistic are the SAME
# binary running the SAME scenarios; only BILOBA_REALISTIC (which swaps b for
# b.Realistic() in the suite) differs. See README.md for methodology + threats.
set -euo pipefail
cd "$(dirname "$0")"

# ---- config (all overridable from the environment) -------------------------
if command -v nproc >/dev/null 2>&1; then DEFAULT_WORKERS=$(nproc); else DEFAULT_WORKERS=$(sysctl -n hw.ncpu); fi
WORKERS="${WORKERS:-$DEFAULT_WORKERS}"
REPS="${REPS:-8}"                 # scenario replication (identical across all three configs)
K="${K:-15}"                      # timed repetitions (after one discarded warmup)
PORT="${PORT:-9889}"
export BASE_URL="${BASE_URL:-http://127.0.0.1:$PORT}"
export REPS
# All three configs run the identical 32 base scenarios. The Biloba binary also
# carries 3 fast-only CSS-hook specs/rep (Bucket F), excluded by label so the count
# matches Playwright. The CSS-vs-locator view lives in buckets.sh.
BASE_SPECS="${BASE_SPECS:-32}"    # A7 + B3 + C5 + D4 + E2 + F3 + G6 + H2
EXPECTED_SPECS=$((BASE_SPECS * REPS))

now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
# prints: median  mean  stddev  min  max  (over all sample args)
stats() { perl -e '@a=sort{$a<=>$b}@ARGV;$n=@a;$m=$n%2?$a[int($n/2)]:($a[$n/2-1]+$a[$n/2])/2;$s+=$_ for @a;$u=$s/$n;$v+=($_-$u)**2 for @a;printf "%6.2f  %6.2f  %6.2f  %6.2f  %6.2f",$m,$u,sqrt($v/$n),$a[0],$a[-1]' "$@"; }

echo "WORKERS=$WORKERS  REPS=$REPS (=> $EXPECTED_SPECS specs/side)  K=$K  BASE_URL=$BASE_URL"

# ---- shared server ----------------------------------------------------------
echo "==> building + starting shared server"
( cd server && go build -o /tmp/cmp-server . )
PORT="$PORT" /tmp/cmp-server >/tmp/cmp-server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf "$BASE_URL/dom.html" >/dev/null 2>&1 && break; sleep 0.1; done
curl -sf "$BASE_URL/dom.html" >/dev/null || { echo "server did not come up"; cat /tmp/cmp-server.log; exit 1; }

# ---- build the Biloba test binary (compilation excluded from timing) --------
echo "==> precompiling Biloba suite (ginkgo build)"
( cd biloba && ginkgo build . >/dev/null )
BILOBA_BIN="$PWD/biloba/biloba.test"

# ---- runner commands --------------------------------------------------------
# biloba-fast and biloba-realistic are the SAME binary + SAME scenarios; only
# BILOBA_REALISTIC differs (it swaps b for b.Realistic() inside the suite). The
# fast-only CSS-hook variants are filtered out so each config matches Playwright.
biloba_fast()   { BILOBA_REALISTIC=0 ginkgo --procs="$1" --label-filter='!csshook' "$BILOBA_BIN"; }
biloba_real()   { BILOBA_REALISTIC=1 ginkgo --procs="$1" --label-filter='!csshook' "$BILOBA_BIN"; }
playwright_run() { ( cd playwright && npx playwright test --workers="$1" --reporter=dot ); }

# ---- spec-count parity check (convince the skeptic counts truly match) ------
echo "==> verifying spec-count parity (expected $EXPECTED_SPECS each)"
BF_COUNT=$(biloba_fast 1 2>/dev/null | grep -Eo 'Ran [0-9]+ of' | grep -Eo '[0-9]+' | tail -1 || true)
BR_COUNT=$(biloba_real 1 2>/dev/null | grep -Eo 'Ran [0-9]+ of' | grep -Eo '[0-9]+' | tail -1 || true)
P_COUNT=$(playwright_run 1 2>/dev/null | grep -Eo '[0-9]+ passed' | grep -Eo '[0-9]+' | tail -1 || true)
echo "    biloba-fast reports:      ${BF_COUNT:-?} specs"
echo "    biloba-realistic reports: ${BR_COUNT:-?} specs"
echo "    playwright reports:       ${P_COUNT:-?} specs"
if [ "${BF_COUNT:-x}" != "$EXPECTED_SPECS" ] || [ "${BR_COUNT:-x}" != "$EXPECTED_SPECS" ] || [ "${P_COUNT:-x}" != "$EXPECTED_SPECS" ]; then
  echo "    !! FATAL: spec counts are not all $EXPECTED_SPECS — comparison would NOT be apples-to-apples"
  exit 1
fi

# ---- browser engines: report the ACTUALLY-LAUNCHED binary on both sides ------
# chromium.executablePath() reports the *headed* Chrome; in headless mode modern
# Playwright launches chrome-headless-shell instead. We surface the real launched
# binary via DEBUG=pw:browser so a skeptic can see both sides run the same class.
echo "==> browser engines (the binary each framework actually launches)"
PW_SHELL=$( cd playwright && DEBUG=pw:browser node -e \
  'const{chromium}=require("@playwright/test");(async()=>{const b=await chromium.launch();await b.close();})()' 2>&1 \
  | grep -Eo '<launching> [^ ]*chrome-headless-shell[^ ]*' | head -1 | sed 's/<launching> //' || true )
echo "    playwright: ${PW_SHELL:-<could not detect>}"
[ -n "${PW_SHELL:-}" ] && echo "                $("$PW_SHELL" --version 2>/dev/null || true)"
BILOBA_SHELL=$(ls -t "$HOME"/.cache/puppeteer/chrome-headless-shell/*/chrome-headless-shell-*/chrome-headless-shell 2>/dev/null | head -1 || true)
[ -z "$BILOBA_SHELL" ] && BILOBA_SHELL=$(command -v chrome-headless-shell || true)
echo "    biloba:     ${BILOBA_SHELL:-<resolved at runtime by biloba>}"
[ -n "${BILOBA_SHELL:-}" ] && echo "                $("$BILOBA_SHELL" --version 2>/dev/null || true)"

# ---- warmup (discarded: primes PW transform cache, browser downloads, OS) ----
echo "==> warmup (discarded)"
biloba_fast "$WORKERS"   >/dev/null 2>&1 || true
biloba_real "$WORKERS"   >/dev/null 2>&1 || true
playwright_run "$WORKERS" >/dev/null 2>&1 || true

# ---- timed, interleaved repetitions ----------------------------------------
echo "==> timing $K repetitions (interleaved)"
declare -a BFPAR BFSER BRPAR BRSER PPAR PSER
timeit() { local s e; s=$(now); "$@" >/dev/null 2>&1; e=$(now); perl -e "printf '%.3f', $e-$s"; }
# Rotate which config leads each rep so none gets a systematic first-mover
# cache/thermal advantage. The configs ALWAYS run sequentially — never concurrently.
for i in $(seq 1 "$K"); do
  case $((i % 3)) in
    1) order="bf br pw";;
    2) order="pw bf br";;
    0) order="br pw bf";;
  esac
  for who in $order; do
    case $who in
      bf) bfp=$(timeit biloba_fast "$WORKERS");  bfs=$(timeit biloba_fast 1);   BFPAR+=("$bfp"); BFSER+=("$bfs");;
      br) brp=$(timeit biloba_real "$WORKERS");  brs=$(timeit biloba_real 1);   BRPAR+=("$brp"); BRSER+=("$brs");;
      pw) ppv=$(timeit playwright_run "$WORKERS"); psv=$(timeit playwright_run 1); PPAR+=("$ppv"); PSER+=("$psv");;
    esac
  done
  printf "    rep %d/%d  fast[par %ss ser %ss]  real[par %ss ser %ss]  pw[par %ss ser %ss]\n" \
    "$i" "$K" "$bfp" "$bfs" "$brp" "$brs" "$ppv" "$psv"
done

# ---- report -----------------------------------------------------------------
echo
echo "=========================== RESULTS (seconds, n=$K) ==========================="
printf "  %-30s %s\n" "config"                                "median    mean  stddev     min     max"
printf "  %-30s %s\n" "biloba-fast      parallel(${WORKERS})" "$(stats "${BFPAR[@]}")"
printf "  %-30s %s\n" "biloba-realistic parallel(${WORKERS})" "$(stats "${BRPAR[@]}")"
printf "  %-30s %s\n" "playwright       parallel(${WORKERS})" "$(stats "${PPAR[@]}")"
printf "  %-30s %s\n" "biloba-fast      serial(1)"            "$(stats "${BFSER[@]}")"
printf "  %-30s %s\n" "biloba-realistic serial(1)"            "$(stats "${BRSER[@]}")"
printf "  %-30s %s\n" "playwright       serial(1)"            "$(stats "${PSER[@]}")"
echo "==============================================================================="
echo "(whole-suite wall clock incl. startup; warm; no retries/artifacts; same server, same specs;"
echo " all three configs run the identical scenarios — biloba-fast via b, biloba-realistic via"
echo " b.Realistic(); runs are sequential and interleaved, never concurrent)"
