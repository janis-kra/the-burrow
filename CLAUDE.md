# Burrow

Go-based personal morning digest email service. Aggregates content from Readwise, Hacker News, Reddit, and weather, then sends a styled HTML email on a cron schedule via Resend API.

## Testing Email Output

To preview the digest HTML without sending an email:

```bash
go run ./cmd/burrow/ --config config.yaml --test
```

This fetches all sources, renders the HTML template, writes it to a temp file, and opens it in the default browser.

To verify the output programmatically with agent-browser:

```bash
agent-browser --allow-file-access open "file:///path/to/burrow-digest-*.html"
agent-browser screenshot --full
agent-browser close
```

## Flags

- `--config <path>` - path to config file (default: `/etc/burrow/config.yaml`)
- `--once` - run once, send the email, and exit
- `--test` - render digest and open HTML in browser (no email sent)
