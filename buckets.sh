#!/usr/bin/env bash
# Three-way per-bucket timing: biloba-fast vs biloba-realistic vs Playwright.
#
# biloba-fast (b) and biloba-realistic (b.Realistic()) are the SAME binary running
# the SAME scenarios — only BILOBA_REALISTIC differs — so every bucket has all three
# columns. Numbers are MARGINAL serial per-spec: each framework's per-invocation
# startup (process + browser launch + suite setup/teardown) is fit and SUBTRACTED
# (the same intercept scaling.sh uses), so a small focused bucket isn't startup-
# dominated. Biloba's startup is config-independent (one shared Chrome), so one
# biloba intercept serves both fast and realistic. The whole-suite parallel story is
# in run.sh; the startup/runtime split is in scaling.sh.
#
# Cells are measured round-robin (every cell once per round, K rounds, median per
# cell) so configs interleave and none gets a systematic thermal edge.
set -euo pipefail
cd "$(dirname "$0")"

REPS="${REPS:-8}"        # replication for the per-bucket measurement
K="${K:-7}"              # rounds (median per cell)
RLO="${RLO:-2}"; RHI="${RHI:-16}"   # the two sizes used to fit each framework's startup intercept
PORT="${PORT:-9889}"
export BASE_URL="${BASE_URL:-http://127.0.0.1:$PORT}"

now(){ perl -MTime::HiRes=time -e 'printf "%.3f",time'; }
med(){ perl -e '@a=sort{$a<=>$b}@ARGV;$n=@a;printf "%.3f",$n%2?$a[int($n/2)]:($a[$n/2-1]+$a[$n/2])/2' "$@"; }
timeit(){ local s e; s=$(now); "$@" >/dev/null 2>&1; e=$(now); perl -e "printf '%.3f',$e-$s"; }

echo "SERIAL per-spec (marginal, startup subtracted)  REPS=$REPS  K=$K  startup-fit sizes=[$RLO,$RHI]  BASE_URL=$BASE_URL"
echo "==> building + starting shared server"
( cd server && go build -o /tmp/cmp-server . )
PORT="$PORT" /tmp/cmp-server >/tmp/cmp-server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf "$BASE_URL/dom.html" >/dev/null 2>&1 && break; sleep 0.1; done
echo "==> precompiling Biloba suite"
( cd biloba && ginkgo build . >/dev/null )
BIN="$PWD/biloba/biloba.test"

# runners — biloba: $1=realistic(0/1) $2=REPS $3=focus $4=labelfilter ; pw: $1=REPS $2=grep
bilo(){ BILOBA_REALISTIC="$1" REPS="$2" ginkgo --procs=1 --focus "$3" --label-filter="$4" "$BIN"; }
play(){ ( cd playwright && REPS="$1" npx playwright test --workers=1 -g "$2" --reporter=dot ); }
FAST='!csshook'

# ---- startup intercept per framework (two-size fit on Page A, 7 specs/rep) ----
echo "==> calibrating per-framework startup intercept (reference: Page A, sizes $RLO & $RHI)"
declare -A C
cadd(){ C[$1]="${C[$1]:-} $2"; }
for i in $(seq 1 "$K"); do
  cadd blo "$(timeit bilo 0 "$RLO" 'Page A' "$FAST")"; cadd bhi "$(timeit bilo 0 "$RHI" 'Page A' "$FAST")"
  cadd plo "$(timeit play "$RLO" 'Page A')";           cadd phi "$(timeit play "$RHI" 'Page A')"
done
intercept(){ perl -e '($wl,$wh,$sl,$sh)=@ARGV; $slope=($wh-$wl)/($sh-$sl); printf "%.4f", $wl-$slope*$sl' "$@"; }
BSTART=$(intercept "$(med ${C[blo]})" "$(med ${C[bhi]})" $((7*RLO)) $((7*RHI)))
PSTART=$(intercept "$(med ${C[plo]})" "$(med ${C[phi]})" $((7*RLO)) $((7*RHI)))
printf "    biloba startup ≈ %ss (serves both fast & realistic)   playwright startup ≈ %ss\n" "$BSTART" "$PSTART"

# ---- round-robin measurement of every cell at the user's REPS ----------------
declare -A SAMP
add(){ SAMP[$1]="${SAMP[$1]:-} $2"; }
bf(){ add "$1" "$(timeit bilo 0 "$REPS" "$2" "$3")"; }   # fast:      key focus labelfilter
br(){ add "$1" "$(timeit bilo 1 "$REPS" "$2" "$3")"; }   # realistic: key focus labelfilter
pw(){ add "$1" "$(timeit play "$REPS" "$2")"; }          # playwright: key grep

echo "==> warmup (discarded)"
bilo 0 "$REPS" "Page A" "$FAST" >/dev/null 2>&1 || true
bilo 1 "$REPS" "Page A" "$FAST" >/dev/null 2>&1 || true
play "$REPS" "Page A" >/dev/null 2>&1 || true

echo "==> measuring $K interleaved rounds"
for i in $(seq 1 "$K"); do
  # fast + realistic bucket aggregates
  bf abF "Page A|Page B" "$FAST"; br abR "Page A|Page B" "$FAST"
  bf cF "Page C" "$FAST";         br cR "Page C" "$FAST"
  bf dF "Bucket D" "$FAST";       br dR "Bucket D" "$FAST"
  bf eF "Bucket E" "$FAST";       br eR "Bucket E" "$FAST"
  bf fF "Bucket F" "$FAST";       br fR "Bucket F" "$FAST"
  bf gF "Bucket G" "$FAST";       br gR "Bucket G" "$FAST"
  bf hF "Bucket H" "$FAST";       br hR "Bucket H" "$FAST"
  bf fcss "Bucket F" "csshook"
  # per-scenario realism (fast + realistic)
  bf e1F "E1 — occlusion" "$FAST";      br e1R "E1 — occlusion" "$FAST"
  bf e2F "E2 — scroll-into-view" "$FAST"; br e2R "E2 — scroll-into-view" "$FAST"
  # playwright
  pw pab "Page A|Page B"; pw pc "Page C"; pw pd "Bucket D"; pw pe "Bucket E"
  pw pf "Bucket F"; pw pg "Bucket G"; pw ph "Bucket H"
  pw pe1 "E1 — occlusion"; pw pe2 "E2 — scroll-into-view"
  printf "    round %d/%d done\n" "$i" "$K"
done

# marginal per-spec ms = (median(wall) - startup) / specs * 1000
bms(){ perl -e "printf '%.1f', ($(med ${SAMP[$1]}) - $BSTART) / ($2 * $REPS) * 1000"; }
pms(){ perl -e "printf '%.1f', ($(med ${SAMP[$1]}) - $PSTART) / ($2 * $REPS) * 1000"; }
rat(){ perl -e "printf '%.1fx', $1/$2"; }

echo
echo "============== 1. PER-BUCKET — three-way marginal per-spec (fast / realistic / pw) =============="
printf "  %-22s %9s %10s %11s   %8s %8s   %s\n" "bucket" "bilo-fast" "bilo-real" "playwright" "pw/fast" "pw/real" "note"
row(){ # label fastkey realkey pwkey specs note
  local f r p; f=$(bms "$2" "$6"); r=$(bms "$3" "$6"); p=$(pms "$4" "$6")
  printf "  %-22s %7sms %8sms %9sms   %8s %8s   %s\n" "$1" "$f" "$r" "$p" "$(rat "$p" "$f")" "$(rat "$p" "$r")" "$5"; }
row "A/B static (reads)"   abF abR pab "no interaction → real≈fast" 10
row "C network"            cF  cR  pc  "interception+latency"       5
row "D scale"              dF  dR  pd  "real DOM weight"            4
row "F semantic locators"  fF  fR  pf  "ARIA engine (see table 3)"  3
row "G vocabulary"         gF  gR  pg  "dbl/right/mid/drag/wheel/tap" 6
row "H pointer options"    hF  hR  ph  "offset + modifier click"    2
row "E realism"            eF  eR  pe  "fast SKIPS work — table 2"  2

echo
echo "============== 2. REALISM per scenario — where fast diverges (marginal per-spec) ================"
printf "  %-22s %9s %10s %11s   %8s %8s   %s\n" "scenario" "bilo-fast" "bilo-real" "playwright" "pw/fast" "pw/real" "fast-track flag"
row "E1 occlusion"      e1F e1R pe1 "divergent-but-passes (clicks through overlay)" 1
row "E2 scroll-in-view" e2F e2R pe2 "divergent-but-passes (no scroll)"              1
echo "   Under fast, bi=b clicks immediately and passes by SKIPPING the wait/scroll a real user needs"
echo "   (on a permanent overlay E1 would pass WRONGLY). Under realistic, bi=b.Realistic() does the same"
echo "   actionability work Playwright does — compare the bilo-real and playwright columns for the fair read."

echo
echo "============== 3. CSS-HOOK vs LOCATOR (Biloba, fast) + Playwright — bucket F ===================="
echo "   Same three F scenarios; bilo-css targets #id, bilo-loc targets a semantic locator."
fcss=$(bms fcss 3); floc=$(bms fF 3); pf=$(pms pf 3)
printf "  %-22s %9s %10s %11s\n" "" "bilo-css" "bilo-loc" "playwright"
printf "  %-22s %7sms %8sms %9sms\n" "F locators (per-spec)" "$fcss" "$floc" "$pf"
printf "  locator tax inside Biloba (loc/css): %s   |   playwright/css: %s\n" "$(rat "$floc" "$fcss")" "$(rat "$pf" "$fcss")"

echo
echo "================================================================================================"
echo "ratio = playwright / biloba (higher = bigger Biloba lead). Marginal serial per-spec (startup"
echo "subtracted); round-robin interleaved; warm; no retries/artifacts; same server; identical"
echo "scenarios per config (fast=b, realistic=b.Realistic()). See SCENARIOS.md for the design."
