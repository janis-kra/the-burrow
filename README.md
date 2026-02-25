# Burrow

Personal morning digest delivered to your inbox. Aggregates Hacker News, weather, Readwise highlights, and Reddit into a single email.

## Setup

```bash
cp config.yaml config.yaml  # edit with your coordinates, subreddit, etc.
export RESEND_API_KEY=re_xxxxxxxxx
export READWISE_API_TOKEN=your_token
```

## Run

```bash
# run once and exit
go run ./cmd/burrow --once --config config.yaml

# run on schedule (default: 7:00 AM daily)
go run ./cmd/burrow --config config.yaml
```

## Container

```bash
# build and run with podman/docker compose
RESEND_API_KEY=re_xxx READWISE_API_TOKEN=xxx podman-compose up -d

# build image only
podman build -t burrow -f Containerfile .

# run container directly
podman run --rm \
  -e RESEND_API_KEY=re_xxx \
  -e READWISE_API_TOKEN=xxx \
  -v ./config.yaml:/etc/burrow/config.yaml:ro \
  burrow
```

## Deploy to Raspberry Pi

Build the binary on your dev machine (cross-compile for ARM):

```bash
# Pi 3 (armv7 / 32-bit)
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o burrow ./cmd/burrow

# Pi 4/5 (arm64 / 64-bit)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o burrow ./cmd/burrow
```

Copy the binary, templates, and config to the Pi (replace `<user>` and `<pi-ip>`):

```bash
scp burrow templates/ config.yaml <user>@<pi-ip>:~/burrow/
```

On the Pi, set env vars and run:

```bash
ssh <user>@<pi-ip>
cd ~/burrow
export RESEND_API_KEY=re_xxxxxxxxx
export READWISE_API_TOKEN=your_token

# test it
./burrow --once --config config.yaml

# run on schedule (stays in foreground)
./burrow --config config.yaml
```

To run on boot, create a systemd service (replace `<user>` with your Pi username):

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

## Test

```bash
go test ./...
```

## Config

All configuration lives in `config.yaml`. Secrets support env var substitution with `${VAR}` or `${VAR:-default}`.

| Key | Description |
|-----|-------------|
| `schedule` | Cron expression for digest timing |
| `email.from` | Sender address (must be verified in Resend) |
| `email.to` | Recipient address |
| `email.resend_api_key` | Resend API key (`${RESEND_API_KEY}`) |
| `weather.latitude/longitude` | Location for weather forecast |
| `readwise.api_token` | Readwise access token |
| `reddit.subreddit` | Subreddit to pull top posts from |
