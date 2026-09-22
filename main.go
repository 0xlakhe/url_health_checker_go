package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed frontend/index.html
var frontendFS embed.FS

type CheckRequest struct {
	URLs        []string `json:"urls"`
	URL         string   `json:"url"`
	TimeoutSecs int      `json:"timeout_secs"`
}

type CheckResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	DurationMs int64  `json:"duration_ms"`
	Duration   string `json:"duration"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

type CheckResponse struct {
	Results []CheckResult `json:"results"`
	TotalMs int64         `json:"total_ms"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("POST /api/check", handleCheck)
	mux.HandleFunc("GET /api/check", handleCheckSingle)

	addr := ":" + port
	fmt.Printf("URL health checker listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := frontendFS.ReadFile("frontend/index.html")
	if err != nil {
		http.Error(w, "frontend not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// GET /api/check?url=https://example.com&timeout=5 — convenience for a single URL.
func handleCheckSingle(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("url"))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ?url= query param"})
		return
	}
	timeout := parseTimeout(r.URL.Query().Get("timeout"), 5)
	results := checkURLs([]string{target}, timeout)
	writeJSON(w, http.StatusOK, CheckResponse{Results: results, TotalMs: totalMs(results)})
}

// POST /api/check {"urls": [...], "url": "...", "timeout_secs": 5}
func handleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read body"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	urls := req.URLs
	if s := strings.TrimSpace(req.URL); s != "" {
		urls = append(urls, s)
	}
	// Also accept newline/comma separated single string entries.
	urls = splitEntries(urls)

	if len(urls) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide urls: [...] or url: \"...\""})
		return
	}
	if len(urls) > 20 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max 20 urls per request"})
		return
	}
	timeout := req.TimeoutSecs
	if timeout <= 0 {
		timeout = 5
	}
	if timeout > 30 {
		timeout = 30
	}

	start := time.Now()
	results := checkURLs(urls, timeout)
	resp := CheckResponse{Results: results, TotalMs: time.Since(start).Milliseconds()}
	writeJSON(w, http.StatusOK, resp)
}

func checkURLs(urls []string, timeoutSecs int) []CheckResult {
	numWorkers := min(5, len(urls))
	jobs := make(chan string, len(urls))
	results := make(chan CheckResult, len(urls))

	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for range numWorkers {
		go func() {
			defer wg.Done()
			for u := range jobs {
				results <- checkURL(u, timeoutSecs)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]CheckResult, 0, len(urls))
	for r := range results {
		out = append(out, r)
	}
	return out
}

func checkURL(rawURL string, secs int) CheckResult {
	target := normalizeURL(rawURL)
	start := time.Now()
	res := CheckResult{URL: target}

	if _, err := url.ParseRequestURI(target); err != nil {
		res.Duration = time.Since(start).String()
		res.Error = "invalid url: " + err.Error()
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(secs)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		res.Duration = time.Since(start).String()
		res.Error = err.Error()
		return res
	}

	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	res.Duration = elapsed.String()
	res.DurationMs = elapsed.Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	res.StatusCode = resp.StatusCode
	res.OK = resp.StatusCode < 400
	if !res.OK {
		res.Error = fmt.Sprintf("bad status: %d", resp.StatusCode)
	}
	return res
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	return s
}

func splitEntries(in []string) []string {
	var out []string
	for _, s := range in {
		for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' }) {
			if t := strings.TrimSpace(f); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

func parseTimeout(s string, def int) int {
	if s == "" {
		return def
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v <= 0 {
		return def
	}
	return min(v, 30)
}

func totalMs(rs []CheckResult) int64 {
	var m int64
	for _, r := range rs {
		m += r.DurationMs
	}
	return m
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
