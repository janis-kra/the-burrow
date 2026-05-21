# Burrow

Personal morning digest delivered to your inbox. Fetches Hacker News, Reddit, Readwise highlights, weather, and Twitter (via Nitter), renders a styled HTML email, and sends it on a cron schedule via [Resend](https://resend.com).

---

## Setup

**Prerequisites:** Go 1.25+, a [Resend](https://resend.com) account with a verified sender domain, and a Readwise account (optional).

```bash
git clone https://github.com/janis-kra/the-burrow
cd the-burrow

# Copy and edit the config
cp config.yaml config.local.yaml
# Edit config.local.yaml: set your email addresses, location coordinates, subreddits, etc.

# Export secrets (never put these in the YAML directly)
export RESEND_API_KEY=re_xxxxxxxxx
export READWISE_API_TOKEN=your_token_here

# Preview the digest in your browser (no email sent)
go run ./cmd/burrow --config config.local.yaml --test

# Send one digest and exit
go run ./cmd/burrow --config config.local.yaml --once

# Run on the cron schedule defined in config (stays in foreground)
go run ./cmd/burrow --config config.local.yaml
```

Run all tests:

```bash
go test ./...
```

---

## Architecture

```
main.go
  │
  ├─ config.go          Load YAML, expand ${ENV_VARS}
  │
  ├─ aggregator.go      Fan out to all fetchers in parallel
  │    ├─ weather.go
  │    ├─ readwise.go
  │    ├─ hackernews.go
  │    ├─ reddit.go
  │    └─ nitter.go
  │
  ├─ renderer.go        Inject results into HTML + text templates
  │    ├─ digest.html
  │    └─ digest.txt
  │
  └─ mailer.go          Send via Resend API
```

**Request flow for one digest:**

1. `main.go` fires (either on cron tick or `--once`)
2. Config is reloaded from disk (picks up any manual edits)
3. `aggregator.FetchAll` fans out to all configured sources concurrently
4. Each fetcher returns `(data any, err error)`; errors don't abort the run
5. `renderer.Render` feeds results into Go templates; failed sources show a "could not load" placeholder
6. `mailer.Send` POSTs to the Resend API with HTML + plain-text body and the header image as an inline attachment
7. Edition counter in `config.yaml` is incremented and written back to disk

---

## Configuration

All config lives in `config.yaml`. Secrets use `${VAR}` or `${VAR:-default}` — substituted from env vars before YAML parsing.

```yaml
schedule: "30 6 * * *"   # standard 5-field cron (6:30 AM daily)
edition: 61               # auto-incremented after each send — do not edit manually

email:
  from: "The Burrow<mail@yourdomain.com>"   # must be verified in Resend
  to: "you@example.com"
  test_to: "sandbox@resend.dev"             # used by --test flag
  resend_api_key: "${RESEND_API_KEY}"

sources:
  - type: weather
    latitude: 52.104
    longitude: 9.357
    name: "Hameln"

  - type: readwise
    api_token: "${READWISE_API_TOKEN}"

  - type: hackernews   # no extra config needed

  - type: reddit
    label: "Dev & Tech"          # section heading in the email
    subreddits: [ClaudeCode, ExperiencedDevs, webdev]

  - type: nitter
    nitter_instance: "https://nitter.net"
    usernames: [bcherny, addyosmani]
    limit: 6                     # max posts to include

  - type: reddit
    label: "General"
    subreddits: [de, DnD]
```

You can have **multiple `reddit` sources** — each becomes its own section in the email. Order in `sources` determines order in the digest.

---

## Deployment

### Container (recommended)

```bash
# Build and start with podman/docker compose
RESEND_API_KEY=re_xxx READWISE_API_TOKEN=xxx podman-compose up -d

# Or build manually
podman build -t burrow -f Containerfile .
podman run --rm \
  -e RESEND_API_KEY=re_xxx \
  -e READWISE_API_TOKEN=xxx \
  -v ./config.yaml:/etc/burrow/config.yaml \
  burrow
```

> **Mount `config.yaml` as writable**, not `:ro`. The app writes back the incremented edition counter after each send.

### Raspberry Pi (binary)

```bash
# Cross-compile on your dev machine
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o burrow ./cmd/burrow

scp burrow templates/ config.yaml user@pi-ip:~/burrow/
```

On the Pi — create a systemd service:

```bash
sudo tee /etc/systemd/system/burrow.service > /dev/null <<'EOF'
[Unit]
Description=Burrow morning digest
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=<user>
WorkingDirectory=/home/<user>/burrow
ExecStart=/home/<user>/burrow/burrow --config /home/<user>/burrow/config.yaml
Environment=RESEND_API_KEY=re_xxxxxxxxx
Environment=READWISE_API_TOKEN=your_token
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now burrow
```

---

## The 5 Files That Matter

### 1. `cmd/burrow/main.go`
The wiring harness. Reads config, constructs fetcher instances, starts the cron scheduler, and handles `--once` / `--test` modes. If you need to add a new source type, this is where you add the `case` that instantiates it. It also owns graceful shutdown: SIGINT/SIGTERM trigger a 2-minute drain window so an in-progress digest can finish.

### 2. `internal/fetcher/fetcher.go`
Defines the `Fetcher` interface (a single `Fetch(ctx context.Context) (any, error)` method) and the `Result` struct the aggregator carries around. Every source implements this interface. The loose `any` return type is intentional — the renderer uses type-assertion helper functions in the template to cast it back.

### 3. `internal/aggregator/aggregator.go`
Runs all fetchers concurrently and collects results. Preserves insertion order regardless of which fetcher finishes first. A fetcher error fills `Result.Error` but doesn't cancel the other goroutines. The 2-minute context deadline is set by the caller (`main.go`), not here.

### 4. `internal/renderer/renderer.go`
The most complex file. Builds a `template.FuncMap` with ~15 helper functions and feeds it into both `digest.html` and `digest.txt`. Key helpers: `hnPosts`, `asRedditPosts`, `asNitterPosts` (type assertions), `excerpt` (sentence truncation), `redditLead`/`redditSidebar` (post selection for the two-column layout), `nextSection` (alternating section counter), `weatherIcon`, and `markdown` (goldmark rendering). If something looks wrong in the rendered email, start here.

### 5. `templates/digest.html`
The email template. Inline CSS only (email clients strip `<style>` tags). The two-column newspaper layout uses a `nextSection` counter — even sections put the lead story on the left, odd sections flip it. Picsum photos are seeded with `{{.DateSeed}}` so the same date always shows the same image. Failed source sections are conditionally swapped for a grey placeholder via `{{if .Error}}`.

---

## 3 Gotchas

### 1. `config.yaml` is mutated at runtime
After every successful send the app increments `edition` and writes it back to `config.yaml` line-by-line (preserving env var placeholders). This means:

- **Never mount the config read-only in a container** — the process will crash trying to write it back.
- **Don't commit config.yaml while the service is running** — you'll create a merge conflict with the edition counter.
- If you need to reset or manually set the edition, stop the service first, edit the file, then restart.

### 2. Readwise highlights are filtered by Reader "archived" status
The Readwise fetcher calls two APIs: the highlights API and the Reader v3 API (to list archived documents). Highlights from Reader documents that are *not yet archived* are silently excluded — the logic assumes you only want highlights from things you've finished reading. Highlights from non-Reader sources (Kindle, manual imports, etc.) are always included.

Symptom: you imported an article to Reader, highlighted it, but it never shows up in digests. Fix: archive the document in Reader, or add it to a non-Reader source type.

### 3. Reddit and Nitter guarantee at least one post per configured account/subreddit
Both the Reddit and Nitter fetchers reserve one slot per configured subreddit/username before filling remaining slots by score/recency. This is intentional but can produce surprising results: if you configure 6 subreddits and `limit` is 5, you'll still get 6 posts (one per sub). The fill logic then sorts the remainder by score. If a subreddit or Nitter user has no recent posts (Nitter enforces a 24-hour window), that source is skipped entirely rather than blocking the run.

---

## How to Add a New Source

Let's say you want to add a `lobsters` source that fetches top posts from Lobste.rs.

**Step 1 — Implement the `Fetcher` interface**

Create `internal/fetcher/lobsters.go`:

```go
package fetcher

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sort"
)

type LobstersPost struct {
    Title    string `json:"title"`
    URL      string `json:"url"`
    Score    int    `json:"score"`
    Author   string `json:"submitter_user"`
    ShortID  string `json:"short_id"`
}

type LobstersFetcher struct{}

func (f *LobstersFetcher) Fetch(ctx context.Context) (any, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
        "https://lobste.rs/hottest.json", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching lobsters: %w", err)
    }
    defer resp.Body.Close()

    var posts []LobstersPost
    if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
        return nil, fmt.Errorf("decoding lobsters response: %w", err)
    }

    sort.Slice(posts, func(i, j int) bool {
        return posts[i].Score > posts[j].Score
    })
    if len(posts) > 5 {
        posts = posts[:5]
    }
    return posts, nil
}
```

**Step 2 — Add a config type**

In `internal/config/config.go`, the `Source` struct uses `Type string` plus loose fields. No changes needed there — your fetcher will be instantiated based on `Type: "lobsters"`, and it needs no config fields.

**Step 3 — Wire it up in `main.go`**

Inside the loop that builds fetchers, add a case:

```go
case "lobsters":
    fetchers = append(fetchers, aggregator.NamedFetcher{
        Name:    "lobsters",
        Label:   "Lobste.rs",
        Fetcher: &fetcher.LobstersFetcher{},
    })
```

**Step 4 — Add template rendering in `renderer.go`**

Add a type-assertion helper to the `FuncMap`:

```go
"asLobstersPosts": func(v any) []fetcher.LobstersPost {
    if posts, ok := v.([]fetcher.LobstersPost); ok {
        return posts
    }
    return nil
},
```

**Step 5 — Add a section to `templates/digest.html`**

Follow the existing HN section as a model. The `{{range}}` over `Results` gives you each source's `Result`. Check `{{eq .Name "lobsters"}}` and render your HTML inside a `{{if not .Error}}` guard.

**Step 6 — Write a test**

Add `internal/fetcher/lobsters_test.go` following the pattern in `hackernews_test.go`: mock the HTTP response with `httptest.NewServer`, call `Fetch`, assert the returned slice has the expected length and is sorted by score.

That's the full loop. The fetcher interface is intentionally minimal — you only implement `Fetch`, and the rest of the pipeline (parallel execution, error isolation, template rendering) handles itself.

---

## Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to config file (default: `/etc/burrow/config.yaml`) |
| `--once` | Run once, send the email, and exit |
| `--test` | Render digest and open HTML in browser — no email sent |
