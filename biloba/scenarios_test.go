package comparison_test

import (
	"strconv"

	"github.com/onsi/biloba"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These are the 34 canonical scenarios from SCENARIOS.md, implemented ONCE as an
// idiomatic Biloba/Ginkgo/Gomega suite. Every interaction goes through `bi`, which
// is `b` for the biloba-fast configuration and `b.Realistic()` for biloba-realistic
// (selected by BILOBA_REALISTIC — see comparison_suite_test.go). So the SAME suite
// runs under both tracks: identical scenarios, identical assertions, only the
// interaction engine swapped. The whole set is replicated REPS times (rep-1 …
// rep-REPS) so the workload is large enough to measure.
//
// Every assertion uses auto-waiting Eventually(...).Should(...) for fairness with
// Playwright's web-first await expect(...). The readiness anchor is always #heading.
//
// A handful of scenarios are realism-sensitive: under the fast track `bi` skips work
// a real user could not skip but still passes the assertion (flagged inline). Under
// the realistic track the same code does the real work. The three-way timings in
// buckets.sh show where that costs.
//
// The one exception to "everything uses bi" is the CSS-hook Bucket F variant
// (Label("csshook")), which deliberately uses fast `b` to measure the
// CSS-vs-locator cost inside Biloba; it is excluded from the headline by label.
var _ = func() bool {
	for r := 1; r <= reps(); r++ {
		Describe("rep-"+strconv.Itoa(r), func() {
			// ---- Page A — dom.html (read-only DOM) ----
			Describe("Page A — dom.html", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/dom.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("A1 — navigation & title", func() {
					Eventually(bi).Should(bi.HaveTitle("DOM Fixture"))
					Eventually("#heading").Should(bi.HaveInnerText("Widgets"))
				})

				It("A2 — count", func() {
					Eventually(".item").Should(bi.HaveCount(4))
				})

				It("A3 — visibility", func() {
					Eventually(".item:not(.hidden)").Should(bi.HaveCount(3))
					Eventually(".item.hidden").ShouldNot(bi.BeVisible())
				})

				It("A4 — inner text of all matches", func() {
					Eventually(".item").Should(bi.EachHaveInnerText("Alpha", "Bravo", "Charlie", "Delta"))
				})

				It("A5 — attribute", func() {
					Eventually("#status").Should(bi.HaveAttribute("data-state", "ready"))
				})

				It("A6 — class", func() {
					Eventually("#status").Should(bi.HaveClass("muted"))
				})

				It("A7 — property", func() {
					Eventually("#docs-link").Should(bi.HaveProperty("href", "https://example.com/docs"))
				})
			})

			// ---- Page B — interactions.html ----
			Describe("Page B — interactions.html", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/interactions.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("B1 — click (counter)", func() {
					Eventually("#increment").Should(bi.Click())
					Eventually("#increment").Should(bi.Click())
					Eventually("#increment").Should(bi.Click())
					Eventually("#count").Should(bi.HaveInnerText("3"))
				})

				It("B2 — form fill (value-set semantics)", func() {
					Eventually("#name").Should(bi.SetValue("Jane"))
					Eventually("#role").Should(bi.SetValue("editor"))
					Eventually("#subscribe").Should(bi.SetValue(true))
					Eventually("#save").Should(bi.Click())
					Eventually("#result").Should(bi.HaveInnerText("Jane / editor / subscribed"))
				})

				It("B3 — real keystroke typing", func() {
					Eventually("#search").Should(bi.Type("ap"))
					Eventually(".fruit:not([style*='display: none'])").Should(bi.HaveCount(2))
					Eventually(".fruit:not([style*='display: none'])").Should(bi.EachHaveInnerText("Apple", "Apricot"))
				})
			})

			// ---- Page C — network.html ----
			Describe("Page C — network.html", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/network.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("C1 — observe a real request", func() {
					Eventually("#load").Should(bi.Click())
					Eventually(".result").Should(bi.HaveCount(3))
					Eventually(".result").Should(bi.EachHaveInnerText("One", "Two", "Three"))
				})

				It("C2 — stub the request", func() {
					b.StubRequest(ContainSubstring("/api/items"), biloba.StubResponse{
						Body:    `{"items":["Stubbed"]}`,
						Headers: map[string]string{"Content-Type": "application/json"},
					})
					Eventually("#load").Should(bi.Click())
					Eventually(".result").Should(bi.HaveCount(1))
					Eventually(".result").Should(bi.EachHaveInnerText("Stubbed"))
				})

				It("C3 — wait through latency", func() {
					Eventually("#load-slow").Should(bi.Click())
					Eventually(".result").Should(bi.HaveCount(3))
				})

				It("C4 — abort the request", func() {
					b.AbortRequest(ContainSubstring("/api/items"))
					Eventually("#load").Should(bi.Click())
					Eventually(".result").Should(bi.HaveCount(1))
					Eventually(".result").Should(bi.EachHaveInnerText("Error"))
				})

				It("C5 — modify the real response", func() {
					// Hits the real server, then rewrites the body (a real round-trip,
					// unlike C2's short-circuit stub).
					b.ModifyResponse(ContainSubstring("/api/items")).WithBody(`{"items":["Modified"]}`)
					Eventually("#load").Should(bi.Click())
					Eventually(".result").Should(bi.HaveCount(1))
					Eventually(".result").Should(bi.EachHaveInnerText("Modified"))
				})
			})

			// ---- Bucket D — speed-at-scale ----
			Describe("Bucket D — scale", func() {
				It("D1 — large table render", func() {
					bi.Navigate(baseURL() + "/scale.html")
					Eventually("#heading").Should(bi.Exist())

					Eventually(".row").Should(bi.HaveCount(1000))
					Eventually(".row[data-id='500'] .name").Should(bi.HaveInnerText("Item 500"))
				})

				It("D2 — filter a large list with real keys", func() {
					bi.Navigate(baseURL() + "/scale.html")
					Eventually("#heading").Should(bi.Exist())

					Eventually("#q").Should(bi.Type("cat-3"))
					Eventually(".row:not([style*='display: none'])").Should(bi.HaveCount(200))
				})

				It("D3 — gated multi-step wizard", func() {
					bi.Navigate(baseURL() + "/wizard.html")
					Eventually("#heading").Should(bi.Exist())

					Eventually("#input1").Should(bi.SetValue("a"))
					Eventually("#next1").Should(bi.Click())
					Eventually("#input2").Should(bi.SetValue("b"))
					Eventually("#next2").Should(bi.Click())
					Eventually("#input3").Should(bi.SetValue("c"))
					Eventually("#next3").Should(bi.Click())
					Eventually("#input4").Should(bi.SetValue("d"))
					Eventually("#finish4").Should(bi.Click())

					Eventually("#summary").Should(bi.HaveInnerText("a-b-c-d"))
				})

				It("D4 — staggered async / eventual consistency", func() {
					bi.Navigate(baseURL() + "/async.html")
					Eventually("#heading").Should(bi.Exist())

					Eventually("#start").Should(bi.Click())
					Eventually(".async-item").Should(bi.HaveCount(10))
					Eventually(".async-item:last-child").Should(bi.HaveInnerText("Item 10"))
				})
			})

			// ---- Bucket E — interaction realism ----
			// Identical code passes under both tracks. Under fast, E1/E4 pass by
			// SKIPPING work (flagged); under realistic the same code does the work.
			Describe("Bucket E — realism", func() {
				It("E1 — occlusion", func() {
					bi.Navigate(baseURL() + "/occlusion.html")
					Eventually("#heading").Should(bi.Exist())

					// fast: el.click() fires through the overlay at ~0ms
					// (divergent-but-passes — a real user could not click yet).
					// realistic: refuses the occluded target; the matcher retries
					// until the overlay clears (~250ms), like Playwright.
					Eventually("#occluded-btn").Should(bi.Click())
					Eventually("#occ-count").Should(bi.HaveInnerText("1"))
				})

				It("E2 — scroll-into-view", func() {
					bi.Navigate(baseURL() + "/scroll.html")
					Eventually("#heading").Should(bi.Exist())

					// fast: el.click() fires without scrolling (divergent-but-passes).
					// realistic: scrolls the button into view, then clicks.
					Eventually("#below-btn").Should(bi.Click())
					Eventually("#scroll-result").Should(bi.HaveInnerText("clicked"))
				})
			})

			// ---- Bucket F — semantic locators ----
			Describe("Bucket F — semantic locators", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/locators.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("F1 — role + name", func() {
					Eventually(bi.ByRole("button").WithName("Save")).Should(bi.Click())
					Eventually("#save-result").Should(bi.HaveInnerText("saved"))
				})

				It("F2 — visible text", func() {
					Eventually(bi.ByText("Featured item")).Should(bi.BeVisible())
				})

				It("F3 — form label", func() {
					Eventually(bi.ByLabel("Email")).Should(bi.SetValue("x@y.com"))
					Eventually(bi.ByLabel("Email")).Should(bi.HaveValue("x@y.com"))
				})
			})

			// CSS-hook sub-comparison (fast-only, uses `b`): same scenarios via #id,
			// to isolate the locator-engine cost inside Biloba. Label-excluded from
			// the headline; consumed only by buckets.sh's CSS-vs-locator table.
			Describe("Bucket F — semantic locators", Label("csshook"), func() {
				BeforeEach(func() {
					b.Navigate(baseURL() + "/locators.html")
					Eventually("#heading").Should(b.Exist())
				})

				It("F1 — role + name", func() {
					Eventually("#save-btn").Should(b.Click())
					Eventually("#save-result").Should(b.HaveInnerText("saved"))
				})

				It("F2 — visible text", func() {
					Eventually("#featured").Should(b.BeVisible())
				})

				It("F3 — form label", func() {
					Eventually("#email").Should(b.SetValue("x@y.com"))
					Eventually("#email").Should(b.HaveValue("x@y.com"))
				})
			})

			// ---- Bucket G — interaction vocabulary ----
			Describe("Bucket G — interaction vocabulary", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/vocab.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("G1 — double-click", func() {
					Eventually("#dbl-btn").Should(bi.DblClick())
					Eventually("#dbl-result").Should(bi.HaveInnerText("double"))
				})

				It("G2 — right-click", func() {
					Eventually("#ctx-btn").Should(bi.RightClick())
					Eventually("#ctx-result").Should(bi.HaveInnerText("menu"))
				})

				It("G3 — middle-click", func() {
					Eventually("#aux-btn").Should(bi.MiddleClick())
					Eventually("#aux-result").Should(bi.HaveInnerText("middle"))
				})

				It("G4 — drag-and-drop", func() {
					Eventually("#drag-src").Should(bi.DragTo("#drop-zone"))
					Eventually("#drop-result").Should(bi.HaveInnerText("dropped"))
				})

				It("G5 — wheel scroll", func() {
					Eventually("#scroll-box").Should(bi.Exist())
					bi.ScrollWheel("#scroll-box", 0, 200)
					Eventually("#wheel-result").Should(bi.HaveInnerText("wheeled"))
				})

				It("G6 — tap (touch)", func() {
					Eventually("#tap-btn").Should(bi.Tap())
					Eventually("#tap-result").Should(bi.HaveInnerText("tapped"))
				})
			})

			// ---- Bucket H — pointer options ----
			Describe("Bucket H — pointer options", func() {
				BeforeEach(func() {
					bi.Navigate(baseURL() + "/pointer.html")
					Eventually("#heading").Should(bi.Exist())
				})

				It("H1 — click at offset", func() {
					Eventually("#click-pad").Should(bi.Click(bi.At(30, 40)))
					Eventually("#click-pad-result").Should(bi.HaveInnerText("ok"))
				})

				It("H2 — modifier-click", func() {
					Eventually("#mod-btn").Should(bi.Click(bi.Shift()))
					Eventually("#mod-result").Should(bi.HaveInnerText("shift"))
				})
			})
		})
	}
	return true
}()
