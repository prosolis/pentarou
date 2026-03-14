# Pentarou — Infrastructure Update Announcer
## Build Plan for Claude Code

## What This Is
Pentarou is a lightweight webhook receiver that listens for Watchtower update notifications
and broadcasts them to a Matrix room. Named after Pentarou the penguin from the Parodius
franchise (Konami).

Watchtower handles everything infrastructure-related: detecting new images, pulling them,
restarting containers, and reporting what changed. Pentarou just has a mouth.

## What It Does
1. Receives Watchtower webhook POST notifications
2. Parses the Watchtower JSON payload
3. Formats the update summary into a readable Matrix message
4. Posts it to the configured Matrix room with retry logic
5. Posts nothing if Watchtower reports no changes

## What It Does NOT Do
Pentarou does not pull images, restart containers, snapshot state, diff containers,
wrap ansible/just commands, or do anything Watchtower already handles. Watchtower
owns the update lifecycle entirely. Pentarou owns the announcement.

## Stack Context
- Runs on a Debian 12 host managed by mash-playbook
- Watchtower runs on the same host, monitoring all mash-playbook containers
- Matrix/Synapse lives on a SEPARATE host — Pentarou posts TO that Matrix instance,
  it does not run alongside it
- Python 3.11+, minimal dependencies

## Architecture
```
src/
  server.py         # lightweight HTTP server, receives Watchtower webhook POSTs
  formatter.py      # turns Watchtower JSON payload into readable Matrix markdown
  notify.py         # posts messages to Matrix room via Client-Server API, with retry logic
  config.py         # loads and validates config.yml, handles env var overrides
systemd/
  pentarou.service  # runs the webhook receiver as a persistent service
config/
  config.example.yml
tests/
  test_formatter.py
  test_notify.py
```

## Watchtower Integration
Watchtower is configured to POST to Pentarou's webhook endpoint after each update run.
In Watchtower's mash-playbook vars.yml:

```yaml
watchtower_notification_url: "generic+http://localhost:8088/webhook"
watchtower_notification_report: true
```

Pentarou listens on localhost only -- no public exposure needed.
Port is configurable, default 8088.

## Watchtower Payload
Watchtower sends a JSON payload on the generic notifications channel. Key fields:

```json
{
  "owner": "Watchtower",
  "title": "Update results",
  "message": "...",
  "level": "info"
}
```

The message field contains the human-readable update summary. Pentarou parses and
reformats this into Matrix markdown rather than forwarding it raw.

If Watchtower reports no updates (all containers already current), Pentarou posts nothing.

## Notification Format
```
🐧 Pentarou — Update Report

Akkoma: updated to sha256:cccddd
PixelFed: updated to sha256:333444
PostgreSQL: updated to sha256:cccddd

2025-03-06 06:04 UTC
```

### No changes
Nothing. Silence is correct when nothing changed.

### On webhook receive error (malformed payload etc.)
Log it, return HTTP 400, do not post to Matrix.

## Matrix Integration
- Uses Matrix Client-Server API directly via stdlib urllib — no SDK
- Posts as a dedicated bot user (@pentarou:yourdomain.com)
- Messages use Matrix markdown formatting (formatted_body)
- Bot user requires an access token with send message permission in the target room

## Retry Logic
- notify.py retries failed Matrix posts up to 3 times with exponential backoff
- Retry failures are logged but never raised
- A failed Matrix post must never cause Pentarou to return an error to Watchtower

## Configuration (config/config.example.yml)
```yaml
matrix:
  homeserver: "https://matrix.yourdomain.com"
  room_id: "!roomid:yourdomain.com"
  # Override with PENTAROU_MATRIX_TOKEN env var — never hardcode in production
  access_token: "YOUR_ACCESS_TOKEN_HERE"
  bot_displayname: "Pentarou"

webhook:
  host: "127.0.0.1"  # localhost only, no public exposure needed
  port: 8088

notifications:
  skip_if_no_changes: true

  # Optional: human-friendly display names for containers
  # Falls back to container name if not listed
  service_names:
    mash-akkoma: "Akkoma"
    mash-pixelfed: "PixelFed"
    mash-lemmy: "Lemmy"
    mash-lemmy-ui: "Lemmy UI"
    mash-writefreely: "WriteFreely"
    mash-funkwhale: "Funkwhale"
    mash-funkwhale-api: "Funkwhale API"
    mash-funkwhale-beat: "Funkwhale Beat"
    mash-postgres: "PostgreSQL"
    mash-redis: "Redis"
    mash-traefik: "Traefik"
    mash-ddclient: "ddclient"
    mash-gitea: "Gitea"
    mash-miniflux: "Miniflux"
```

## Systemd Service
Pentarou runs as a persistent service, not a timer. It's always listening for Watchtower
webhooks. A single service unit is all that's needed:

```ini
[Unit]
Description=Pentarou — Infrastructure Update Announcer
After=network.target

[Service]
Type=simple
User=pentarou
EnvironmentFile=/opt/pentarou/.env
WorkingDirectory=/opt/pentarou
ExecStart=/opt/pentarou/.venv/bin/python -m src
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Future Feature (not MVP)
Changelog summarization: fetch upstream release notes per updated service,
summarize via LLM API, include inline with the Matrix notification.
The formatter should accept an optional `changelog_summary` field per service
so this can be added later without restructuring.

## Dependencies
- pyyaml — config loading
- pytest — tests
- Everything else: Python stdlib only (http.server for webhook, urllib for Matrix)

## Naming Convention — Bot Ecosystem
- **Bellhop** — media request portal
- **GwinBee** — gaming deals aggregator
- **Melora** — webhook receiver / *arr stack notifications with Matrix threading
- **Pentarou** — this bot (infrastructure update announcements)

All names drawn from the TwinBee and Parodius franchises (Konami).
