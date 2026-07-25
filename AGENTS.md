# AGENTS.md

## What this is

Web crawler for 蚌埠市住建局 (bengbu zjj) news articles. Scrapes ~371 paginated list pages, fetches each article detail, stores in SQLite.

## Commands

```bash
go build -o crawler .           # build (includes server)
./crawler                        # run crawler (default DB: ./bengbu_wxjj.db)
./crawler -server                # start Web UI on http://localhost:8080
DB_PATH=/tmp/db.db ./crawler     # use custom DB path
PORT=9090 ./crawler -server      # Web UI on custom port
```

No test/lint/typecheck tooling configured.

## Key behavior

- Hardcoded 371 pages at `https://zjj.bengbu.gov.cn/content/column/6801691?pageIndex=N`
- Skips articles already in DB (dedup by URL)
- Rate limiting: 800ms between detail fetches, 1500ms between list pages
- Attempts 8 content selectors in order, falls back to `div.rightnr`
- `INSERT OR REPLACE` — re-fetches overwrite existing rows
- SQLite DB is auto-created with table on first run
- `amount` column extracted from content via regex `(金额|拨款金额|维修金额)[：:]\s*([0-9,]+)`

## Architecture

Single `main.go` (~210 lines), standard library + `goquery` for HTML parsing + `go-sqlite3` (CGO, requires gcc).