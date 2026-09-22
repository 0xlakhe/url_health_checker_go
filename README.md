# URL Health Checker (Go)

Small web app: paste website(s) into the page (or call the API) and get UP/DOWN, status code, and latency. Go stdlib only — `net/http` server + worker pool + a single static page.

## Run

```bash
go run .
# open http://localhost:8080
PORT=8080 go run .   # override port
```

## API

**POST /api/check** — check up to 20 URLs:

```bash
curl -X POST localhost:8080/api/check \
  -H 'Content-Type: application/json' \
  -d '{"urls":["https://google.com","example.com"],"timeout_secs":5}'
```

```json
{
  "results": [
    {"url":"https://google.com","status_code":200,"duration_ms":120,"duration":"120ms","ok":true},
    {"url":"https://example.com","status_code":200,"duration_ms":60,"duration":"60ms","ok":true}
  ],
  "total_ms": 135
}
```

Fields: `url` (single-URL shorthand also accepted), `timeout_secs` (default 5, max 30).

**GET /api/check?url=example.com&timeout=5** — single-URL convenience.

Notes: scheme auto-added (`example.com` → `https://example.com`), `ok` means status &lt; 400 with no error, newline/comma-separated entries are split.

## How it works

- `main.go` — `http.ServeMux` with `GET /` (embedded `frontend/index.html`), `POST /api/check`, `GET /api/check`
- `checkURLs()` — worker pool (`min(5, len(urls))` workers, buffered channels, `sync.WaitGroup`)
- `checkURL()` — `GET` with per-request `context.WithTimeout`, discards body (4KB cap), records code + latency
- `frontend/index.html` — no-deps page: textarea (one URL/line), timeout input, results table

## Build / checks

```bash
go build -o urlchecker .
go vet ./...
gofmt -l .   # must output nothing
```

## Limits / next steps

- Max 20 URLs per request, 30s max timeout, 64KB request cap.
- `http.DefaultClient` — add custom transport/retries if you need TLS tuning.
- No tests yet; single `main` package is fine until a second consumer appears.
