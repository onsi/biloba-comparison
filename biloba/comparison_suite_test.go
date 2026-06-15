package comparison_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/onsi/biloba"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestComparison(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Biloba Comparison Suite")
}

// b is the root tab (always fast). bi is the interaction handle every scenario
// uses — it is `b` for the biloba-fast configuration and `b.Realistic()` for the
// biloba-realistic configuration, selected by the BILOBA_REALISTIC env var. The
// SAME 34 scenarios run under both: fast = `b`, realistic = `b.Realistic()`,
// nothing else changes. (b stays available for the fast-only CSS-hook variants.)
var b *biloba.Biloba
var bi *biloba.Biloba

var _ = SynchronizedBeforeSuite(func() {
	biloba.SpinUpChrome(GinkgoT(), biloba.AutoInstallHeadlessShell())
}, func() {
	// Align the max auto-wait timeout with Playwright's default expect timeout (5s)
	// so neither side is more/less patient than the other. Polling interval stays at
	// Gomega's default 10ms. This does NOT affect happy-path timing (Eventually
	// returns as soon as the condition holds) — it only governs robustness under
	// load, where Gomega's 1s default would otherwise make Biloba flakier than
	// Playwright on the heavier D/E specs.
	SetDefaultEventuallyTimeout(5 * time.Second)
	b = biloba.ConnectToChrome(GinkgoT())
	if os.Getenv("BILOBA_REALISTIC") == "1" {
		bi = b.Realistic()
	} else {
		bi = b
	}
})

var _ = BeforeEach(func() {
	b.Prepare()
}, OncePerOrdered)

// baseURL is the shared comparison server, read from the environment.
func baseURL() string {
	if url := os.Getenv("BASE_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:9889"
}

// reps is the replication factor that fans the 34 base scenarios (A/B/C/D/E/F/G/H)
// out into enough independent, parallelizable work.
func reps() int {
	if r := os.Getenv("REPS"); r != "" {
		if n, err := strconv.Atoi(r); err == nil && n > 0 {
			return n
		}
	}
	return 8
}
