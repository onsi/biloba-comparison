#!/usr/bin/env bash
# Separates FIXED STARTUP cost from MARGINAL PER-SPEC RUNTIME for each framework.
#
# Method (identical for both frameworks, so it's neutral): run the SAME workload at
# several sizes (REPS => 13*REPS specs), take the median wall-clock at each size,
# then least-squares fit  time = startup + perSpec * specs.  The intercept is the
# fixed startup (process + browser launch + suite setup + teardown); the slope is
# the marginal cost of one more spec.  R^2 reports how linear the data is (a good
# fit validates the model). This relies on NEITHER framework's empty-suite quirks.
set -euo pipefail
cd "$(dirname "$0")"

if command -v nproc >/dev/null 2>&1; then DEFAULT_WORKERS=$(nproc); else DEFAULT_WORKERS=$(sysctl -n hw.ncpu); fi
WORKERS="${WORKERS:-$DEFAULT_WORKERS}"
REPS_LIST="${REPS_LIST:-1 2 4 8 16}"   # workload sizes (specs = 32 * each)
K="${K:-5}"                            # timed reps per size (after a warmup)
PORT="${PORT:-9889}"
BASE_SPECS="${BASE_SPECS:-32}"         # base scenarios per rep — see SCENARIOS.md
REF_SPECS="${REF_SPECS:-256}"          # size at which to print the startup/runtime split (32*8)
export BASE_URL="${BASE_URL:-http://127.0.0.1:$PORT}"

now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
median() { perl -e '@a=sort{$a<=>$b}@ARGV;$n=@a;printf "%.3f",$n%2?$a[int($n/2)]:($a[$n/2-1]+$a[$n/2])/2' "$@"; }

echo "WORKERS=$WORKERS  REPS_LIST=[$REPS_LIST]  K=$K  BASE_URL=$BASE_URL"

echo "==> building + starting shared server"
( cd server && go build -o /tmp/cmp-server . )
PORT="$PORT" /tmp/cmp-server >/tmp/cmp-server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf "$BASE_URL/dom.html" >/dev/null 2>&1 && break; sleep 0.1; done

echo "==> precompiling Biloba suite"
( cd biloba && ginkgo build . >/dev/null )
BILOBA_BIN="$PWD/biloba.test"; [ -f biloba/biloba.test ] && BILOBA_BIN="$PWD/biloba/biloba.test"

# All three configs run the identical 32 base scenarios (CSS-hook variants excluded).
# biloba-fast = b; biloba-realistic = b.Realistic() (same binary, BILOBA_REALISTIC env).
biloba_fast()    { BILOBA_REALISTIC=0 ginkgo --procs="$1" --label-filter='!csshook' "$BILOBA_BIN"; }
biloba_real()    { BILOBA_REALISTIC=1 ginkgo --procs="$1" --label-filter='!csshook' "$BILOBA_BIN"; }
playwright_run() { ( cd playwright && npx playwright test --workers="$1" --reporter=dot ); }
timeit() { local s e; s=$(now); "$@" >/dev/null 2>&1; e=$(now); perl -e "printf '%.3f', $e-$s"; }

echo "==> warmup (discarded)"
REPS=2 biloba_fast "$WORKERS"   >/dev/null 2>&1 || true
REPS=2 biloba_real "$WORKERS"   >/dev/null 2>&1 || true
REPS=2 playwright_run "$WORKERS" >/dev/null 2>&1 || true

bfpar_pts=""; brpar_pts=""; ppar_pts=""; bfser_pts=""; brser_pts=""; pser_pts=""
printf "\n  %-6s %-6s | %-8s %-8s %-9s | %-8s %-8s %-9s\n" \
  "REPS" "specs" "bf-par" "br-par" "pw-par" "bf-ser" "br-ser" "pw-ser"
for N in $REPS_LIST; do
  export REPS=$N; specs=$((BASE_SPECS * N))
  bfp=(); brp=(); pp=(); bfs=(); brs=(); ps=()
  for i in $(seq 1 "$K"); do
    bfp+=("$(timeit biloba_fast "$WORKERS")"); brp+=("$(timeit biloba_real "$WORKERS")"); pp+=("$(timeit playwright_run "$WORKERS")")
    bfs+=("$(timeit biloba_fast 1)");          brs+=("$(timeit biloba_real 1)");          ps+=("$(timeit playwright_run 1)")
  done
  bfpm=$(median "${bfp[@]}"); brpm=$(median "${brp[@]}"); ppm=$(median "${pp[@]}")
  bfsm=$(median "${bfs[@]}"); brsm=$(median "${brs[@]}"); psm=$(median "${ps[@]}")
  bfpar_pts+=" $specs $bfpm"; brpar_pts+=" $specs $brpm"; ppar_pts+=" $specs $ppm"
  bfser_pts+=" $specs $bfsm"; brser_pts+=" $specs $brsm"; pser_pts+=" $specs $psm"
  printf "  %-6s %-6s | %-8s %-8s %-9s | %-8s %-8s %-9s\n" \
    "$N" "$specs" "${bfpm}s" "${brpm}s" "${ppm}s" "${bfsm}s" "${brsm}s" "${psm}s"
done

report() { # report "name" x y x y ...   (points passed as separate args)
  local name="$1"; shift
  REF="$REF_SPECS" perl -e '
    $name=shift @ARGV; @p=@ARGV; $n=@p/2;
    for($i=0;$i<$n;$i++){$x=$p[2*$i];$y=$p[2*$i+1];$sx+=$x;$sy+=$y;$sxx+=$x*$x;$sxy+=$x*$y;$syy+=$y*$y;}
    $b=($n*$sxy-$sx*$sy)/($n*$sxx-$sx*$sx); $a=($sy-$b*$sx)/$n;
    $d=($n*$sxx-$sx*$sx)*($n*$syy-$sy*$sy); $r=$d>0?($n*$sxy-$sx*$sy)/sqrt($d):1;
    $ref=$ENV{REF}; $rt=$b*$ref;
    printf "  %-32s startup %6.2fs   per-spec %6.1fms   runtime\@%d %5.2fs   total\@%d %5.2fs   R^2 %.4f\n",
           $name,$a,$b*1000,$ref,$rt,$ref,$a+$rt,$r*$r;
  ' "$name" "$@"
}
echo
echo "============ STARTUP vs RUNTIME (linear fit: time = startup + perSpec*specs) ============"
report "biloba-fast      parallel(${WORKERS})" $bfpar_pts
report "biloba-realistic parallel(${WORKERS})" $brpar_pts
report "playwright       parallel(${WORKERS})" $ppar_pts
report "biloba-fast      serial(1)"            $bfser_pts
report "biloba-realistic serial(1)"            $brser_pts
report "playwright       serial(1)"            $pser_pts
echo "========================================================================================"
echo "startup = fixed cost (process + browser launch + suite setup/teardown), the y-intercept."
echo "per-spec = marginal cost of one more spec, the slope. runtime@N = per-spec * N specs."
echo "High R^2 (>0.98) means the linear startup+perspec model fits — trust the split."
