# Pentarou

A diligent penguin that announces infrastructure updates to your Matrix room.

Pentarou is a lightweight webhook receiver that listens for [Watchtower](https://containrrr.dev/watchtower/) update notifications and broadcasts them to a Matrix room. Watchtower handles the entire update lifecycle -- pulling images, restarting containers, reporting what changed. Pentarou just has a mouth.

Named after Pentarou the penguin from the Parodius franchise (Konami). Part of the TwinBee/Parodius community bot ecosystem.

## Siblings

- **Bellhop** -- media request portal
- **GwinBee** -- gaming deals aggregator
- **Melora** -- webhook receiver / *arr stack notifications with Matrix threading

## Requirements

- Python 3.11+
- A Matrix bot user with message-send permission in the target room
- Watchtower configured to send generic webhook notifications
- systemd (Pentarou runs as a persistent service)

## Installation

### 1. Clone and configure

```bash
git clone https://github.com/prosolis/Pentarou.git
cd Pentarou
cp config/config.example.yml config/config.yml
```

Edit `config/config.yml` with your actual values:

```yaml
matrix:
  homeserver: "https://matrix.yourdomain.com"
  room_id: "!roomid:yourdomain.com"
  access_token: "YOUR_ACCESS_TOKEN_HERE"
  bot_displayname: "Pentarou"

webhook:
  host: "127.0.0.1"
  port: 8088

notifications:
  skip_if_no_changes: true
  service_names:
    mash-akkoma: "Akkoma"
    mash-pixelfed: "PixelFed"
    mash-postgres: "PostgreSQL"
```

### 2. Set up a virtual environment

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

### 3. Set the Matrix token

The access token can live in `config.yml` for local testing, but in production, keep it out of version control:

```bash
export PENTAROU_MATRIX_TOKEN="syt_your_real_token_here"
```

For systemd, put it in `/opt/pentarou/.env`:

```bash
PENTAROU_MATRIX_TOKEN=syt_your_real_token_here
```

The env var always wins over whatever is in the config file.

### 4. Configure Watchtower

In your Watchtower configuration (e.g. mash-playbook `vars.yml`):

```yaml
watchtower_notification_url: "generic+http://localhost:8088/webhook"
watchtower_notification_report: true
```

### 5. Install systemd service

```bash
sudo cp systemd/pentarou.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pentarou.service
```

## Usage

### Manual run

```bash
# Start the webhook server
python -m src

# With an alternate config file
python -m src -c /etc/pentarou/config.yml
```

### Test with curl

```bash
# Simulate a Watchtower update notification
curl -X POST http://localhost:8088/webhook \
  -H "Content-Type: application/json" \
  -d '{"title":"Watchtower updates","message":"Updating /mash-akkoma (sha256:aaa111 to sha256:bbb222)","level":"info"}'

# Simulate no updates (should be silent)
curl -X POST http://localhost:8088/webhook \
  -H "Content-Type: application/json" \
  -d '{"title":"Watchtower updates","message":"No containers need updating","level":"info"}'
```

## How It Works

1. Watchtower completes an update run and POSTs a JSON payload to `localhost:8088/webhook`
2. Pentarou parses the `message` field from the Watchtower payload
3. Extracts container names and digest changes from the update report
4. Formats a readable Matrix message with human-friendly service names
5. Posts it to the configured Matrix room (with retry logic)
6. If nothing was updated, posts nothing -- silence is correct

## What Pentarou Says

### Containers updated

```
🐧 Pentarou — Update Report

- Akkoma: sha256:aaabbbcccddd → sha256:777888999000
- PixelFed: sha256:555666777888 → sha256:999000aaabbb
- PostgreSQL: sha256:cccdddeeefff → sha256:000111222333

2025-03-06 06:04 UTC
```

### No changes

Nothing. Silence. The room is blissfully unbothered.

### Malformed payload

Logged server-side, HTTP 400 returned to Watchtower. Nothing posted to Matrix.

## Matrix Setup

Pentarou talks to Matrix directly via the Client-Server API using Python's stdlib `urllib`. No SDK, no extra dependencies.

1. Create a bot user on your homeserver (e.g. `@pentarou:yourdomain.com`)
2. Generate an access token for the bot
3. Invite the bot to the target room and have it join
4. Set the `room_id` in config (the internal ID like `!abc123:yourdomain.com`, not the alias)

The bot only needs `m.room.message` send permission.

## Project Structure

```
Pentarou/
├── src/
│   ├── __main__.py      # Entry point -- starts webhook server
│   ├── server.py        # HTTP server, receives Watchtower webhook POSTs
│   ├── formatter.py     # Parses Watchtower JSON into Matrix markdown
│   ├── notify.py        # Matrix posting via urllib, 3x retry with backoff
│   └── config.py        # YAML config + PENTAROU_MATRIX_TOKEN env override
├── config/
│   └── config.example.yml
├── systemd/
│   └── pentarou.service
├── tests/
│   ├── test_formatter.py
│   ├── test_notify.py
│   └── test_server.py
├── requirements.txt
└── README.md
```

## Configuration Reference

| Key | Required | Default | Description |
|---|---|---|---|
| `matrix.homeserver` | Yes | -- | Base URL of your Matrix homeserver |
| `matrix.room_id` | Yes | -- | Internal room ID for announcements |
| `matrix.access_token` | Yes | -- | Bot access token (prefer `PENTAROU_MATRIX_TOKEN` env var) |
| `matrix.bot_displayname` | No | `Pentarou` | Display name for the bot |
| `webhook.host` | No | `127.0.0.1` | Bind address for the webhook server |
| `webhook.port` | No | `8088` | Port for the webhook server |
| `notifications.skip_if_no_changes` | No | `true` | Post nothing when Watchtower reports no updates |
| `notifications.service_names` | No | `{}` | Map of container name to human-friendly display name |

## Running Tests

```bash
source .venv/bin/activate
python -m pytest tests/ -v
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `FileNotFoundError: config/config.yml` | Config file doesn't exist yet | `cp config/config.example.yml config/config.yml` and edit it |
| `PENTAROU_MATRIX_TOKEN` not taking effect | Env var not exported, or `.env` path wrong in systemd unit | Verify `EnvironmentFile=` in the service unit points to your actual `.env` |
| Matrix post returns 403 | Bot not in the room or lacks permission | Invite the bot and make sure it has send-message permission |
| Matrix post returns 401 | Access token is wrong or expired | Regenerate the token, update config or env var |
| Watchtower not sending webhooks | Notification URL misconfigured | Verify `watchtower_notification_url` uses `generic+http://` prefix |
| No notification after update | Watchtower reported no changes, or message format not recognized | `curl` the webhook endpoint manually to test; check Pentarou logs |
| Port 8088 already in use | Another service on that port | Change `webhook.port` in config |

## License

See repository for license details.
