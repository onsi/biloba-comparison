// Command server is the single shared target for both the Biloba and Playwright
// comparison suites. It serves identical static fixtures and a tiny JSON API to
// both frameworks so the server cancels out of the measurement entirely.
//
// It is intentionally dependency-free (stdlib only) and deterministic: the slow
// endpoint sleeps a fixed duration so network-bound scenarios cost the same on
// both sides.
package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed fixtures/*.html
var fixturesFS embed.FS

// items is the canonical payload returned by the API. Both suites assert against
// exactly these values.
var items = map[string][]string{"items": {"One", "Two", "Three"}}

// slowDelay is the fixed latency of the slow endpoint. It is framework-neutral:
// both suites pay the same wall-clock waiting on it.
const slowDelay = 300 * time.Millisecond

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9889"
	}

	sub, err := fs.Sub(fixturesFS, "fixtures")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))

	writeItems := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(items)
	}
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		writeItems(w)
	})
	mux.HandleFunc("/api/slow-items", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(slowDelay)
		writeItems(w)
	})

	addr := "127.0.0.1:" + port
	log.Printf("comparison server listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
