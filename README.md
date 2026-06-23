# linkrot

[![CI](https://github.com/andreaisabelmontana/linkrot/actions/workflows/ci.yml/badge.svg)](https://github.com/andreaisabelmontana/linkrot/actions/workflows/ci.yml)
![license: MIT](https://img.shields.io/badge/license-MIT-blue)
![go](https://img.shields.io/badge/go-1.26-00ADD8)

A fast, concurrent broken-link checker. Point it at a site; it crawls the same-host pages, gathers every link and asset they reference, checks them all in parallel, and reports what's broken — and *which page* points at it. Works as a one-line command or a CI gate.

## Install

```bash
go install github.com/andreaisabelmontana/linkrot@latest
# or, from a clone:
go build -o linkrot .
```

Only dependency: `golang.org/x/net/html` for robust HTML tokenising. Everything else is the standard library.

## Usage

```bash
# Crawl a site and list broken links, worst-first
linkrot https://example.com

# Only show what's broken, and fail the shell (CI gate)
linkrot --only-broken --fail-on-error https://example.com

# Don't check external sites, crawl two levels deep, 32 workers
linkrot --internal-only --depth 2 --concurrency 32 https://example.com

# Machine-readable
linkrot --format json https://example.com
```

### Flags

| Flag | Default | Meaning |
|------|--------:|---------|
| `--concurrency` | 16 | max simultaneous requests |
| `--depth` | 3 | how many link-hops to crawl from the start page (same host) |
| `--internal-only` | false | skip external links instead of checking them |
| `--timeout` | 15 | per-request timeout (seconds) |
| `--only-broken` | false | hide ok/skipped rows in table output |
| `--fail-on-error` | false | exit non-zero if any link is broken |
| `--format` | table | `table` or `json` |

## How it works

1. **Crawl** — a bounded-concurrency BFS over same-host HTML pages (up to `--depth`), recording for every referenced URL the set of pages that link to it. Assets (`.png`, `.css`, …) are checked but not crawled into.
2. **Check** — every unique URL is verified concurrently with a worker pool, preferring a cheap `HEAD` and falling back to `GET` when a server rejects it. 2xx/3xx is OK; 4xx/5xx or a transport error is broken.
3. **Report** — results sorted broken-first, with the referencing pages, as a table or JSON.

Concurrency is a single knob (`--concurrency`) enforced by a semaphore over both phases; `Ctrl-C` cancels cleanly via `context`.

## Structure

```
main.go                       CLI flags, orchestration, exit codes
internal/linkrot/
  crawler.go                  BFS crawl + concurrent check (the core)
  extract.go                  HTML link/asset extraction
  check.go                    HEAD-then-GET reachability check
  report.go                   table / json / summary
  types.go                    Result and Status
  *_test.go                   httptest-backed tests (no network)
```

## License

MIT — see [LICENSE](LICENSE).
