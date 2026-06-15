// Command charts renders the comparison's SVG figures (stdlib only). Two subcommands:
//
//	charts pertest <fast.json> <real.json> <pw.json> <outdir>
//	    scatter.svg  per-test log-log scatter of Biloba (fast + realistic) vs Playwright,
//	                 with a y=x diagonal and 2x/4x reference lines. Below the diagonal =
//	                 Biloba faster; the vertical offset is the speedup. One point per
//	                 scenario (A1…H2), keyed by the code, which is identical across suites.
//	    buckets.svg  grouped bars of per-bucket median per-test duration (fast / realistic /
//	                 playwright) with P25–P75 whiskers. Per-spec body time only (no startup).
//
//	charts config <summary.json> <outdir>
//	    config.svg   per-config whole-suite wall clock, parallel and serial, each bar split
//	                 into fixed startup (from the scaling fit) + spec runtime, with +/-stddev
//	                 error bars from the run.sh repeats.
//
// Ginkgo per-spec time is SpecReport.RunTime; Playwright's is result.duration. Both are the
// marginal per-test cost (no suite startup) — startup lives only in config.svg.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "pertest":
		if len(os.Args) != 6 {
			usage()
		}
		pertest(os.Args[2], os.Args[3], os.Args[4], os.Args[5])
	case "config":
		if len(os.Args) != 4 {
			usage()
		}
		configCmd(os.Args[2], os.Args[3])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  charts pertest <fast.json> <real.json> <pw.json> <outdir>")
	fmt.Fprintln(os.Stderr, "  charts config <summary.json> <outdir>")
	os.Exit(2)
}

// ============================ pertest: scatter + buckets ============================

type ginkgoReport struct {
	SpecReports []struct {
		LeafNodeText string `json:"LeafNodeText"`
		LeafNodeType string `json:"LeafNodeType"`
		State        string `json:"State"`
		RunTime      int64  `json:"RunTime"` // nanoseconds
	} `json:"SpecReports"`
}

type pwReport struct {
	Suites []pwSuite `json:"suites"`
}
type pwSuite struct {
	Title  string    `json:"title"`
	Suites []pwSuite `json:"suites"`
	Specs  []pwSpec  `json:"specs"`
}
type pwSpec struct {
	Title string `json:"title"`
	Tests []struct {
		Results []struct {
			Duration float64 `json:"duration"` // ms
			Status   string  `json:"status"`
		} `json:"results"`
	} `json:"tests"`
}

var codeRe = regexp.MustCompile(`^([A-H][0-9]+)`)

func code(title string) string {
	if m := codeRe.FindStringSubmatch(strings.TrimSpace(title)); m != nil {
		return m[1]
	}
	return ""
}

func loadGinkgo(path string) map[string][]float64 {
	raw, err := os.ReadFile(path)
	must(err)
	var reps []ginkgoReport
	must(json.Unmarshal(raw, &reps))
	acc := map[string][]float64{}
	for _, rep := range reps {
		for _, s := range rep.SpecReports {
			if s.LeafNodeType != "It" || s.State != "passed" {
				continue
			}
			if c := code(s.LeafNodeText); c != "" {
				acc[c] = append(acc[c], float64(s.RunTime)/1e6) // ns -> ms
			}
		}
	}
	return acc
}

func loadPW(path string) map[string][]float64 {
	raw, err := os.ReadFile(path)
	must(err)
	var rep pwReport
	must(json.Unmarshal(raw, &rep))
	acc := map[string][]float64{}
	var walk func(s pwSuite)
	walk = func(s pwSuite) {
		for _, sp := range s.Specs {
			if c := code(sp.Title); c != "" {
				for _, t := range sp.Tests {
					for _, r := range t.Results {
						if r.Status == "passed" {
							acc[c] = append(acc[c], r.Duration)
						}
					}
				}
			}
		}
		for _, sub := range s.Suites {
			walk(sub)
		}
	}
	for _, s := range rep.Suites {
		walk(s)
	}
	return acc
}

type point struct {
	code           string
	bucket         string
	fast, real, pw float64 // per-scenario medians
}

func bucketOf(c string) string {
	if c[0] == 'A' || c[0] == 'B' {
		return "A/B"
	}
	return string(c[0])
}

func pertest(fastPath, realPath, pwPath, outdir string) {
	fast, real, pw := loadGinkgo(fastPath), loadGinkgo(realPath), loadPW(pwPath)
	var pts []point
	for c, f := range fast {
		r, okr := real[c]
		p, okp := pw[c]
		if !okr || !okp {
			continue
		}
		pts = append(pts, point{c, bucketOf(c), median(f), median(r), median(p)})
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].code < pts[j].code })
	must(os.WriteFile(outdir+"/scatter.svg", []byte(scatterSVG(pts, fast, real, pw)), 0o644))
	must(os.WriteFile(outdir+"/buckets.svg", []byte(bucketsSVG(fast, real, pw)), 0o644))
	n := 0
	for _, v := range fast {
		n = len(v)
		break
	}
	fmt.Printf("wrote %s/scatter.svg and %s/buckets.svg (%d scenarios, ~%d samples each)\n", outdir, outdir, len(pts), n)
	printStats(fast, real, pw)
}

// printStats dumps per-bucket and per-scenario mean ± SEM (ms) for each config, so the
// README's bucket table can carry the same numbers the charts show.
func printStats(fast, real, pw map[string][]float64) {
	ms := func(v []float64) string {
		m := meanOf(v)
		return fmt.Sprintf("%.1f±%.1f", m, sdOf(v, m)/math.Sqrt(float64(len(v))))
	}
	pool := func(m map[string][]float64, pred func(string) bool) []float64 {
		var all []float64
		for c, v := range m {
			if pred(c) {
				all = append(all, v...)
			}
		}
		return all
	}
	fmt.Println("\nper-bucket mean±SEM (ms)   fast / realistic / playwright")
	for _, bk := range []string{"A/B", "C", "D", "E", "F", "G", "H"} {
		bk := bk
		p := func(c string) bool { return bucketOf(c) == bk }
		fmt.Printf("  %-4s  %s  /  %s  /  %s\n", bk, ms(pool(fast, p)), ms(pool(real, p)), ms(pool(pw, p)))
	}
	fmt.Println("per-scenario (E):")
	for _, sc := range []string{"E1", "E2"} {
		sc := sc
		p := func(c string) bool { return c == sc }
		fmt.Printf("  %-4s  %s  /  %s  /  %s\n", sc, ms(pool(fast, p)), ms(pool(real, p)), ms(pool(pw, p)))
	}
}

// ============================ config: startup vs runtime ============================

type summary struct {
	Workers int `json:"workers"`
	Specs   int `json:"specs"`
	Rows    []struct {
		Config  string  `json:"config"`
		Mode    string  `json:"mode"`
		Startup float64 `json:"startup"`
		Total   float64 `json:"total"`
		Stddev  float64 `json:"stddev"`
	} `json:"rows"`
}

func configCmd(summaryPath, outdir string) {
	raw, err := os.ReadFile(summaryPath)
	must(err)
	var s summary
	must(json.Unmarshal(raw, &s))
	must(os.WriteFile(outdir+"/config.svg", []byte(configSVG(s)), 0o644))
	fmt.Printf("wrote %s/config.svg\n", outdir)
}

// ============================ helpers ============================

func median(xs []float64) float64 { return pct(xs, 0.5) }

func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

func sdOf(xs []float64, mean float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	s := 0.0
	for _, v := range xs {
		s += (v - mean) * (v - mean)
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func pct(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	idx := p * float64(len(s)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	return s[lo] + (s[hi]-s[lo])*(idx-float64(lo))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ============================ theme ============================

const (
	colFast    = "#1f9d55" // green  — biloba-fast
	colReal    = "#2b6cb0" // blue   — biloba-realistic
	colPW      = "#dd6b20" // orange — playwright
	colStartup = "#c9ced6" // grey   — fixed startup segment
	colAxis    = "#444"
	colGrid    = "#e6e6e6"
	colDiag    = "#999"
	font       = "font-family='-apple-system,Helvetica,Arial,sans-serif'"
)

func colorFor(cfg string) string {
	switch {
	case strings.Contains(cfg, "realistic"):
		return colReal
	case strings.Contains(cfg, "fast"):
		return colFast
	default:
		return colPW
	}
}

// ============================ scatter (log-log) ============================

func scatterSVG(pts []point, fast, real, pw map[string][]float64) string {
	const W, H = 760, 560
	const ml, mr, mt, mb = 78, 150, 56, 64
	x0, x1 := float64(ml), float64(W-mr)
	y0, y1 := float64(H-mb), float64(mt)

	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		for _, v := range []float64{p.pw, p.fast, p.real} {
			if v > 0 {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	loD, hiD := math.Floor(math.Log10(lo)), math.Ceil(math.Log10(hi))
	lg := func(v float64) float64 { return (math.Log10(v) - loD) / (hiD - loD) }
	sx := func(v float64) float64 { return x0 + lg(v)*(x1-x0) }
	sy := func(v float64) float64 { return y0 + lg(v)*(y1-y0) }

	var b strings.Builder
	svgHead(&b, W, H)
	fmt.Fprintf(&b, `<text x='%d' y='28' font-size='16' font-weight='bold' fill='#222'>Per-test timing — Biloba vs Playwright</text>`, ml)
	fmt.Fprintf(&b, `<text x='%d' y='46' fill='#666'>log–log; serial per-test median (ms). Below the diagonal = Biloba faster.</text>`, ml)

	for d := loD; d <= hiD; d++ {
		v := math.Pow(10, d)
		px, py := sx(v), sy(v)
		line(&b, px, y0, px, y1, colGrid, 1, "")
		line(&b, x0, py, x1, py, colGrid, 1, "")
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='middle' fill='%s'>%s</text>`, px, y0+18, colAxis, num(v))
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='end' fill='%s'>%s</text>`, x0-8, py+4, colAxis, num(v))
	}
	line(&b, x0, y0, x1, y0, colAxis, 1.5, "")
	line(&b, x0, y0, x0, y1, colAxis, 1.5, "")
	fmt.Fprintf(&b, `<text x='%.1f' y='%d' text-anchor='middle' fill='%s'>Playwright per-test (ms)</text>`, (x0+x1)/2, H-22, colAxis)
	fmt.Fprintf(&b, `<text transform='translate(22,%.1f) rotate(-90)' text-anchor='middle' fill='%s'>Biloba per-test (ms)</text>`, (y0+y1)/2, colAxis)

	for _, ref := range []struct {
		k    float64
		dash string
		lbl  string
	}{{1, "", "y=x"}, {2, "5 4", "2×"}, {4, "2 4", "4×"}} {
		x0v, x1v := math.Pow(10, loD), math.Pow(10, hiD)
		line(&b, sx(x0v), sy(x0v/ref.k), sx(x1v), sy(x1v/ref.k), colDiag, 1.2, ref.dash)
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' fill='%s' font-size='11'>%s</text>`, sx(x1v)-22, sy(x1v/ref.k)-5, colDiag, ref.lbl)
	}

	// light P25–P75 error whiskers (both axes), drawn under the markers
	const colErr = "#cfcfcf"
	for _, p := range pts {
		xLo, xHi := sx(pct(pw[p.code], 0.25)), sx(pct(pw[p.code], 0.75))
		for _, s := range []struct {
			yv     float64
			lo, hi float64
		}{
			{p.fast, sy(pct(fast[p.code], 0.25)), sy(pct(fast[p.code], 0.75))},
			{p.real, sy(pct(real[p.code], 0.25)), sy(pct(real[p.code], 0.75))},
		} {
			line(&b, xLo, sy(s.yv), xHi, sy(s.yv), colErr, 1, "")   // x-spread (Playwright)
			line(&b, sx(p.pw), s.lo, sx(p.pw), s.hi, colErr, 1, "") // y-spread (Biloba)
		}
	}
	for _, p := range pts {
		fmt.Fprintf(&b, `<circle cx='%.1f' cy='%.1f' r='4.5' fill='%s' fill-opacity='0.9'/>`, sx(p.pw), sy(p.fast), colFast)
		fmt.Fprintf(&b, `<rect x='%.1f' y='%.1f' width='8' height='8' fill='%s' fill-opacity='0.9'/>`, sx(p.pw)-4, sy(p.real)-4, colReal)
	}

	lx, ly := x1+24, y1+6
	fmt.Fprintf(&b, `<circle cx='%.1f' cy='%.1f' r='5' fill='%s'/><text x='%.1f' y='%.1f' fill='#333'>biloba-fast</text>`, lx, ly, colFast, lx+12, ly+4)
	fmt.Fprintf(&b, `<rect x='%.1f' y='%.1f' width='9' height='9' fill='%s'/><text x='%.1f' y='%.1f' fill='#333'>biloba-realistic</text>`, lx-4, ly+18, colReal, lx+12, ly+26)
	fmt.Fprintf(&b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='%s'/><text x='%.1f' y='%.1f' fill='#777' font-size='11'>P25–P75</text>`, lx-4, ly+40, lx+8, ly+40, colErr, lx+14, ly+44)
	fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' fill='#777' font-size='11'>each point = one</text>`, lx-5, ly+66)
	fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' fill='#777' font-size='11'>scenario (A1…H2),</text>`, lx-5, ly+82)
	fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' fill='#777' font-size='11'>x = its Playwright time</text>`, lx-5, ly+98)
	b.WriteString(`</svg>`)
	return b.String()
}

// ============================ buckets (grouped bars + IQR) ============================

func bucketsSVG(fast, real, pw map[string][]float64) string {
	order := []string{"A/B", "C", "D", "E", "F", "G", "H"}
	names := map[string]string{"A/B": "A/B static", "C": "C network", "D": "D scale", "E": "E realism", "F": "F locators", "G": "G vocab", "H": "H pointer"}
	pool := func(m map[string][]float64, bk string) []float64 {
		var all []float64
		for c, v := range m {
			if bucketOf(c) == bk {
				all = append(all, v...)
			}
		}
		return all
	}
	// bar = MEAN per-test (matches comparison.md's per-spec table: total/count); whisker
	// = ±SEM (precision of that mean). SEM stays small even for buckets that mix fast and
	// slow scenarios (C latency, D async, E occlusion wait) — the scenario-level spread is
	// shown in the scatter, not conflated into these bars.
	type cell struct{ mean, sem float64 }
	type row struct {
		name    string
		f, r, p cell
	}
	mk := func(v []float64) cell {
		m := meanOf(v)
		return cell{m, sdOf(v, m) / math.Sqrt(float64(len(v)))}
	}
	var rows []row
	maxv := 0.0
	for _, bk := range order {
		f, r, p := mk(pool(fast, bk)), mk(pool(real, bk)), mk(pool(pw, bk))
		rows = append(rows, row{names[bk], f, r, p})
		for _, c := range []cell{f, r, p} {
			maxv = math.Max(maxv, c.mean+c.sem)
		}
	}

	// horizontal: 7 bucket groups stacked top-to-bottom, 3 bars each (fast/real/pw)
	const W, H = 760, 520
	const ml, mr, top = 132, 28, 80
	x0, x1 := float64(ml), float64(W-mr)
	nm := niceMax(maxv)
	sx := func(v float64) float64 { return x0 + (v/nm)*(x1-x0) }

	const barH, inPitch, groupGap = 13, 15, 13
	groupPitch := float64(3*inPitch + groupGap)
	yBot := float64(top) + groupPitch*float64(len(rows)) - groupGap

	var b strings.Builder
	svgHead(&b, W, H)
	fmt.Fprintf(&b, `<text x='40' y='28' font-size='16' font-weight='bold' fill='#222'>Per-bucket per-test duration (ms)</text>`)
	fmt.Fprintf(&b, `<text x='40' y='46' fill='#666'>serial; bar = mean per-test (matches the per-spec table), whisker = ±SEM. Shorter is faster.</text>`)

	// x gridlines + axis labels
	for i := 0; i <= 4; i++ {
		v := nm * float64(i) / 4
		px := sx(v)
		line(&b, px, float64(top-6), px, yBot, colGrid, 1, "")
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='middle' fill='%s' font-size='11'>%s</text>`, px, yBot+16, colAxis, num(v))
	}
	line(&b, x0, float64(top-6), x0, yBot, colAxis, 1.5, "")

	for i, rw := range rows {
		gy := float64(top) + float64(i)*groupPitch
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='end' font-weight='bold' fill='#333' font-size='12'>%s</text>`, x0-10, gy+1.5*inPitch+1, rw.name)
		for j, cc := range []struct {
			c   cell
			col string
		}{{rw.f, colFast}, {rw.r, colReal}, {rw.p, colPW}} {
			cy := gy + float64(j)*inPitch + barH/2
			xe := sx(cc.c.mean)
			fmt.Fprintf(&b, `<rect x='%.1f' y='%.1f' width='%.1f' height='%d' fill='%s'/>`, x0, cy-barH/2, xe-x0, barH, cc.col)
			herrbar(&b, sx(cc.c.mean-cc.c.sem), sx(cc.c.mean+cc.c.sem), cy)
			barLabel(&b, x0, xe, cy, num(cc.c.mean))
		}
	}

	legendH(&b, 40, float64(H-16), []legItem{{colFast, "biloba-fast"}, {colReal, "biloba-realistic"}, {colPW, "playwright"}})
	b.WriteString(`</svg>`)
	return b.String()
}

// ============================ config (stacked startup+runtime, stddev bars) ==========

func configSVG(s summary) string {
	const W, H = 820, 360
	const ml, mr, top = 132, 28, 78
	x0, x1 := float64(ml), float64(W-mr)

	// rows: parallel group (fast/realistic/playwright) above, serial group below — one shared x-scale
	order := []string{"biloba-fast", "biloba-realistic", "playwright"}
	type rowD struct {
		cfg                string
		startup, total, sd float64
	}
	get := func(mode string) []rowD {
		out := make([]rowD, 0, 3)
		for _, cfg := range order {
			for _, r := range s.Rows {
				if r.Mode == mode && r.Config == cfg {
					out = append(out, rowD{r.Config, r.Startup, r.Total, r.Stddev})
				}
			}
		}
		return out
	}
	groups := []struct {
		title string
		rows  []rowD
	}{
		{fmt.Sprintf("parallel — %d workers", s.Workers), get("parallel")},
		{"serial — 1 worker", get("serial")},
	}
	maxv := 0.0
	for _, g := range groups {
		for _, r := range g.rows {
			maxv = math.Max(maxv, r.total+r.sd)
		}
	}
	nm := niceMax(maxv)
	sx := func(v float64) float64 { return x0 + (v/nm)*(x1-x0) }

	var b strings.Builder
	svgHead(&b, W, H)
	fmt.Fprintf(&b, `<text x='40' y='28' font-size='16' font-weight='bold' fill='#222'>Whole-suite wall clock — startup vs spec runtime</text>`)
	fmt.Fprintf(&b, `<text x='40' y='48' fill='#666'>same scale; bar = total split into fixed startup + spec runtime (scaling fit); whisker = ±stddev over the repeats.</text>`)

	const barH, pitch, groupGap = 20, 26, 26
	// plot bottom = top + parallel(3 pitch) + groupGap + heading + serial(3 pitch)
	yBot := top + 3*pitch + groupGap + 20 + 3*pitch + 6

	// x gridlines + axis labels (shared scale)
	for i := 0; i <= 4; i++ {
		v := nm * float64(i) / 4
		px := sx(v)
		line(&b, px, float64(top-6), px, float64(yBot), colGrid, 1, "")
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='middle' fill='%s' font-size='11'>%ss</text>`, px, float64(yBot)+16, colAxis, num(v))
	}
	line(&b, x0, float64(yBot), x1, float64(yBot), colAxis, 1.5, "")

	cursor := float64(top)
	for _, g := range groups {
		fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' font-weight='bold' fill='#333' font-size='12'>%s</text>`, float64(28), cursor+12, g.title)
		cursor += 20
		for _, r := range g.rows {
			cy := cursor + barH/2
			xs := sx(r.startup)
			xt := sx(r.total)
			// startup (grey, left) then runtime (color)
			fmt.Fprintf(&b, `<rect x='%.1f' y='%.1f' width='%.1f' height='%d' fill='%s'/>`, x0, cursor, xs-x0, barH, colStartup)
			fmt.Fprintf(&b, `<rect x='%.1f' y='%.1f' width='%.1f' height='%d' fill='%s'/>`, xs, cursor, xt-xs, barH, colorFor(r.cfg))
			// horizontal stddev whisker at the total
			herrbar(&b, sx(r.total-r.sd), sx(r.total+r.sd), cy)
			// labels: config name at left, total centered in the bar, startup inside the grey
			fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='end' fill='%s' font-size='12'>%s</text>`, x0-8, cy+4, colAxis, strings.TrimPrefix(r.cfg, "biloba-"))
			barLabel(&b, x0, xt, cy, num(r.total)+"s")
			if xs-x0 > 26 {
				fmt.Fprintf(&b, `<text x='%.1f' y='%.1f' text-anchor='middle' fill='#555' font-size='9'>%ss</text>`, (x0+xs)/2, cy+3, num(r.startup))
			}
			cursor += pitch
		}
		cursor += groupGap
	}

	legendH(&b, 28, float64(H-14), []legItem{{colStartup, "startup (fixed)"}, {colFast, "fast runtime"}, {colReal, "realistic runtime"}, {colPW, "playwright runtime"}})
	b.WriteString(`</svg>`)
	return b.String()
}

// ============================ svg primitives ============================

func svgHead(b *strings.Builder, w, h int) {
	fmt.Fprintf(b, `<svg xmlns='http://www.w3.org/2000/svg' width='%d' height='%d' viewBox='0 0 %d %d' %s font-size='13'>`, w, h, w, h, font)
	fmt.Fprintf(b, `<rect width='%d' height='%d' fill='white'/>`, w, h)
}
func line(b *strings.Builder, x1, y1, x2, y2 float64, col string, w float64, dash string) {
	d := ""
	if dash != "" {
		d = fmt.Sprintf(` stroke-dasharray='%s'`, dash)
	}
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='%s' stroke-width='%.1f'%s/>`, x1, y1, x2, y2, col, w, d)
}
func whisker(b *strings.Builder, cx, yLo, yHi float64) { // yLo is the P25 (lower on screen=higher y), yHi P75
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#333' stroke-width='1'/>`, cx, yLo, cx, yHi)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#333' stroke-width='1'/>`, cx-3, yLo, cx+3, yLo)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#333' stroke-width='1'/>`, cx-3, yHi, cx+3, yHi)
}
func errbar(b *strings.Builder, cx, yLo, yHi float64) {
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, cx, yLo, cx, yHi)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, cx-4, yLo, cx+4, yLo)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, cx-4, yHi, cx+4, yHi)
}

// herrbar is a horizontal ±error bar centred on a horizontal-bar row.
func herrbar(b *strings.Builder, xLo, xHi, cy float64) {
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, xLo, cy, xHi, cy)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, xLo, cy-4, xLo, cy+4)
	fmt.Fprintf(b, `<line x1='%.1f' y1='%.1f' x2='%.1f' y2='%.1f' stroke='#222' stroke-width='1.3'/>`, xHi, cy-4, xHi, cy+4)
}

// barLabel writes a value label for a horizontal bar from xStart..xEnd: centered
// inside the bar (white) when it fits, otherwise just past the bar end (dark).
func barLabel(b *strings.Builder, xStart, xEnd, cy float64, text string) {
	tw := float64(len(text))*7 + 8
	if xEnd-xStart >= tw {
		fmt.Fprintf(b, `<text x='%.1f' y='%.1f' text-anchor='middle' fill='white' font-weight='bold' font-size='11'>%s</text>`, (xStart+xEnd)/2, cy+4, text)
	} else {
		fmt.Fprintf(b, `<text x='%.1f' y='%.1f' fill='#222' font-weight='bold' font-size='11'>%s</text>`, xEnd+5, cy+4, text)
	}
}

type legItem struct{ col, name string }

func legend(b *strings.Builder, x, y float64, items []legItem) {
	for k, it := range items {
		yy := y + float64(k)*20
		fmt.Fprintf(b, `<rect x='%.1f' y='%.1f' width='12' height='12' fill='%s'/><text x='%.1f' y='%.1f' fill='#333'>%s</text>`, x, yy, it.col, x+18, yy+11, it.name)
	}
}

// legendH lays the legend out horizontally, advancing by a fixed slot wide enough
// for the longest label.
func legendH(b *strings.Builder, x, y float64, items []legItem) {
	const slot = 165
	for k, it := range items {
		xx := x + float64(k)*slot
		fmt.Fprintf(b, `<rect x='%.1f' y='%.1f' width='12' height='12' fill='%s'/><text x='%.1f' y='%.1f' fill='#333'>%s</text>`, xx, y, it.col, xx+18, y+11, it.name)
	}
}

func niceMax(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 1.5, 2, 2.5, 3, 4, 5, 7.5, 10} {
		if exp*m >= v {
			return exp * m
		}
	}
	return exp * 10
}

func num(v float64) string {
	switch {
	case v >= 10:
		return fmt.Sprintf("%.0f", v)
	case v >= 1:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}
