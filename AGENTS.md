# AGENTS.md — url_health_checker_go

## Project summary

Small Go web app (`module urlchecker`, `go 1.26.4`, stdlib only): `net/http` server + worker pool that checks user-submitted URLs. `GET /` serves embedded `frontend/index.html`; `POST /api/check` and `GET /api/check?url=` return JSON health results.

## Structure

- `main.go` — server (`ServeMux`), `checkURLs()` pool, `checkURL()`, JSON types
- `frontend/index.html` — no-deps UI (textarea, timeout, results table), embedded via `//go:embed`
- `go.mod` — module definition only
- `Makefile` — `run` / `build` / `vet` / `fmt` / `clean`
- `README.md` — usage and API contract

## Build / run / verify

```bash
go run .
go build -o urlchecker .
go vet ./...
gofmt -l .        # must output nothing
make vet fmt run
```

Verified with `go1.27.1`: `go vet ./...` clean, `go build` succeeds.

## Conventions

- Standard `gofmt` formatting is required — run `gofmt -w .` before committing.
- Keep the worker-pool pattern: buffered `jobs`/`results` channels sized to input, `sync.WaitGroup`, closer goroutine for `results`.
- Timeouts via `context.WithTimeout` per request (currently 2s). Don't use bare `http.Get`.
- Always `defer resp.Body.Close()` after a successful `Do`.
- `log` for errors, `fmt` for result output (existing split — keep it).

## What to watch for

- API limits are intentional: max 20 URLs, 64KB body, timeout clamped 1–30s — keep them when extending.
- `checkURL()` normalizes missing scheme to `https://` and treats status < 400 as OK; don't silently change either without updating frontend + README.
- `CheckResult.Error` must always be populated on failure (DNS, timeout, bad status) — never return bare status 0 without it; frontend renders this column.
- `http.DefaultClient` has no redirect/transport tuning — configure explicitly if adding retries or custom TLS.
- URLs, worker count, and timeout are hardcoded — prefer CLI flags (`flag` package) over more constants when extending.

## Don't

- Don't add external dependencies for stdlib-solvable work (`net/http`, `flag`, `encoding/json` cover current needs).
- Don't commit binaries (`urlchecker`, `*.out`, `*.test`) — covered by `.gitignore`.
- Don't restructure into multiple packages until there's a second consumer (e.g. tests or a library import).
