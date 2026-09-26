# AGENTS.md

## What this is

Crawler + web UI for 蚌埠市住建局 (bengbu zjj) 维修资金拨付公示 articles. One Go package (`module bengbu_crawler`), two files: `main.go` (crawler, amount extraction, email) and `server.go` (HTTP API + inline HTML/JS UI as the `pageHTML` string constant). No tests, no lint, no CI.

## Commands

```bash
go build -o crawler .      # build; pure Go, no CGO/gcc needed
./crawler                  # default: Web UI + crawl loop (see Scheduling)
./crawler -server          # Web UI only, never crawls
./crawler -once            # one crawl round then exit — but still binds the web port first
```

Env vars: `DB_PATH` (default `./bengbu_wxjj.db`), `PORT` (default **20173**, not 8080), `SMTP_EMAIL` / `SMTP_PASS`, `BASE_URL` (unsubscribe link base).

## Scheduling (6h loop, not cron)

- `main()` runs `crawlAll` forever with `time.Sleep(6 * time.Hour)` between rounds (`main.go:63`). `-server` disables crawling entirely.
- Each round: empty DB → all 371 list pages; otherwise only the first 3 (`main.go:87`). A failed list page is skipped after 3s (lost until next round).
- `notifySubscribers` runs once at the end of every round: emails matched articles (`id > last_article_id AND title LIKE %community%`). `last_article_id` advances to the round max only when there were no matches or the send succeeded — send failure leaves the cursor in place so the next round retries the same range.
- Gotcha: new/re-subscribed rows start with `last_article_id = 0`, so the first notify round emails the subscriber **all historical matches**.

## Key behavior

- SQLite driver is **`modernc.org/sqlite` (pure Go)** — README's claim of `go-sqlite3`/CGO/gcc is stale; trust `go.mod`.
- Dedup by `url`: existing articles are skipped before fetching, so `INSERT OR REPLACE` almost never rewrites a row.
- `amount`: first sums the `拨付金额` column of HTML tables; falls back to regexes (`维修资金列支金额`, `拨付金额`, `￥`) only for values in 100 < v < 50,000,000 (`main.go:328`).
- `cleanContent` (`main.go:251`) reconstructs the 序号/房号/维修内容/拨付金额 text table into pipe-separated rows — source-site table format changes must be handled there too.
- Detail parsing tries 8 selectors in order, falls back to `div.rightnr`; list pages parse `ul.doc_list li`.
- Subscribe API caps email/community at 30 chars and rejects `sensitiveWords` (`main.go:433`); re-subscribing clears `unsubscribed_at` and resets the token.
- SMTP credentials have hardcoded defaults in `main.go:27` — env vars override; never commit new secrets to source.
- API routes: `/api/articles`, `/api/article?id=`, `/api/subscribe`, `/api/unsubscribe?token=`, `/api/subscriptions`.
- `.gitignore` covers the `crawler` binary, `*.db*`, and `*.log`.
