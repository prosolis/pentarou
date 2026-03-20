# Pentarou

A diligent penguin that announces infrastructure updates to your Matrix room.

Pentarou is a lightweight webhook receiver that listens for [Watchtower](https://github.com/nicholas-fedor/watchtower) update notifications, fetches release notes from GitHub, and posts per-service update messages to a Matrix room. Watchtower handles the entire update lifecycle -- pulling images, restarting containers, reporting what changed. Pentarou enriches those reports with context and delivers them where your team is already looking.

Named after Pentarou the penguin from the Parodius franchise (Konami). Part of the TwinBee/Parodius community bot ecosystem.

## Siblings

- **Bellhop** -- media request portal
- **GwinBee** -- gaming deals aggregator
- **Melora** -- webhook receiver / *arr stack notifications with Matrix threading

## Requirements

- Go 1.21+ (build only -- the binary has no runtime dependencies)
- A Matrix bot user with message-send permission in the target room
- [Watchtower](https://github.com/nicholas-fedor/watchtower) configured with `json.v1` notification template
- systemd (Pentarou runs as a persistent service)

## Installation

### 1. Build

```bash
git clone https://github.com/prosolis/Pentarou.git
cd Pentarou
go build -o pentarou .
```

### 2. Configure Pentarou

```bash
cp config/config.example.yml config/config.yml
```

Edit `config/config.yml` with your actual values:

```yaml
matrix:
  homeserver: "https://matrix.yourdomain.com"
  room_id: "!roomid:yourdomain.com"
  username: "@pentarou:yourdomain.com"
  bot_displayname: "Pentarou"

webhook:
  host: "0.0.0.0"
  port: 8088

notifications:
  skip_if_no_changes: true
  watchtower_updates_room: "!updates:yourdomain.com"
```

### 3. Set secrets via environment variables

Pentarou supports two authentication methods:

**Option A: Username + password (recommended)**

Pentarou logs in automatically, stores the access token in `data/device.json` with restrictive permissions (0600), and reuses it across restarts. If the token expires, it re-authenticates automatically. This also enables cross-signing so the bot's device is automatically trusted.

```bash
export PENTAROU_MATRIX_PASSWORD="your-bot-password"
```

**Option B: Explicit access token**

If you prefer to manage the token yourself:

```bash
export PENTAROU_MATRIX_TOKEN="syt_your_real_token_here"
```

You need one or the other -- either username+password or an access token.

For higher GitHub API rate limits (optional -- unauthenticated allows 60 requests/hour, authenticated allows 5,000):

```bash
export PENTAROU_GITHUB_TOKEN="ghp_your_token_here"
```

For webhook authentication (recommended when binding to `0.0.0.0`):

```bash
export PENTAROU_WEBHOOK_TOKEN="your-secret-token-here"
```

When set, all requests to `/webhook` must include an `Authorization: Bearer <token>` header. Requests without a valid token receive a 401 response. If left unset, the webhook accepts all requests.

For systemd, put all secrets in `/opt/pentarou/.env`:

```bash
PENTAROU_MATRIX_PASSWORD=your-bot-password
PENTAROU_GITHUB_TOKEN=ghp_your_token_here
PENTAROU_WEBHOOK_TOKEN=your-secret-token-here
```

Environment variables always override values in the config file.

### 4. Configure Watchtower

Pentarou requires Watchtower's **`json.v1` notification template**, which provides structured per-container update data instead of plain text summaries.

#### Docker Compose

Watchtower runs inside Docker and cannot reach the host's `localhost`. Use `host.docker.internal` to route to the host machine, and set Pentarou's bind address to `0.0.0.0` so it accepts connections from the Docker bridge network.

Pentarou config:

```yaml
webhook:
  host: "0.0.0.0"  # required -- 127.0.0.1 won't accept traffic from Docker
  port: 8088
```

Watchtower service:

```yaml
services:
  watchtower:
    image: ghcr.io/nicholas-fedor/watchtower:latest
    environment:
      WATCHTOWER_NOTIFICATION_URL: "generic+http://host.docker.internal:8088/webhook?Authorization=Bearer+your-secret-token-here"
      WATCHTOWER_NOTIFICATION_TEMPLATE: "json.v1"
      WATCHTOWER_MONITOR_ONLY: "true"  # recommended: detect updates without applying them
    extra_hosts:
      - "host.docker.internal:host-gateway"  # needed on Linux; macOS/Windows have this by default
```

The `Authorization=Bearer+your-secret-token-here` query parameter tells Shoutrrr's generic webhook to send the `Authorization` header with each request. Use the same token value you set in `PENTAROU_WEBHOOK_TOKEN`.

#### MASH Playbook (Ansible)

If you're using the [MASH playbook](https://github.com/mother-of-all-self-hosting/mash-playbook), add to your `vars.yml`:

```yaml
watchtower_environment_variables_additional_variables: |
  WATCHTOWER_NOTIFICATION_URL=generic+http://host.docker.internal:8088/webhook?Authorization=Bearer+your-secret-token-here
  WATCHTOWER_NOTIFICATION_TEMPLATE=json.v1
  WATCHTOWER_MONITOR_ONLY=true
```

You'll also need to ensure `host.docker.internal` resolves to the host. If your Watchtower container's compose file doesn't already include `extra_hosts`, add it via the playbook's container configuration.

#### What these settings do

| Variable | Purpose |
|---|---|
| `WATCHTOWER_NOTIFICATION_URL` | Shoutrrr URL pointing to Pentarou's webhook endpoint. `generic+http://` tells Shoutrrr to POST the notification as the raw HTTP body. Uses `host.docker.internal` so the container can reach the host. |
| `WATCHTOWER_NOTIFICATION_TEMPLATE` | **Must be `json.v1`**. This makes Watchtower emit a structured JSON payload with a `report` object containing per-container update details, instead of a plain text summary. |
| `WATCHTOWER_MONITOR_ONLY` | Optional but recommended. Watchtower detects available updates and notifies without pulling or restarting. |

#### Payload format

With `json.v1`, Watchtower sends a structured payload. Pentarou reads the `report.updated` array:

```json
{
  "title": "Watchtower updates on hostname",
  "host": "hostname",
  "entries": [],
  "report": {
    "scanned": [],
    "updated": [
      {
        "id": "c79110000000",
        "name": "akkoma-akkoma-1",
        "imageName": "ghcr.io/akkoma-im/akkoma:latest",
        "currentImageId": "sha256:abc123...",
        "latestImageId": "sha256:def456...",
        "state": "Updated"
      }
    ],
    "failed": [],
    "skipped": [],
    "stale": [],
    "fresh": []
  }
}
```

Pentarou also handles the case where Shoutrrr wraps the json.v1 output inside a `{"message": "..."}` envelope -- both delivery formats work automatically.

### 5. Deploy

```bash
sudo cp pentarou /opt/pentarou/
sudo cp config/config.yml /opt/pentarou/config/
sudo cp systemd/pentarou.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pentarou.service
```

## Usage

### Manual run

```bash
# Start the webhook server (default config path: config/config.yml)
./pentarou

# With an alternate config file
./pentarou -config /etc/pentarou/config.yml
```

### Test with curl

```bash
# Simulate a Watchtower update with a mapped container (will try to fetch release notes)
curl -X POST http://localhost:8088/webhook \
  -H "Authorization: Bearer your-secret-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Watchtower updates",
    "host": "testhost",
    "report": {
      "updated": [
        {
          "name": "akkoma-akkoma-1",
          "imageName": "ghcr.io/akkoma-im/akkoma:latest",
          "currentImageId": "sha256:aaa111",
          "latestImageId": "sha256:bbb222",
          "state": "Updated"
        }
      ],
      "scanned": [], "failed": [], "skipped": [], "stale": [], "fresh": []
    }
  }'

# Simulate an unmapped container (posts minimal notification without release notes)
curl -X POST http://localhost:8088/webhook \
  -H "Authorization: Bearer your-secret-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Watchtower updates",
    "host": "testhost",
    "report": {
      "updated": [
        {
          "name": "akkoma-db-1",
          "imageName": "postgres:16",
          "currentImageId": "sha256:ccc333",
          "latestImageId": "sha256:ddd444",
          "state": "Updated"
        }
      ],
      "scanned": [], "failed": [], "skipped": [], "stale": [], "fresh": []
    }
  }'

# Simulate no updates (should be silent)
curl -X POST http://localhost:8088/webhook \
  -H "Authorization: Bearer your-secret-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Watchtower updates",
    "host": "testhost",
    "report": {
      "updated": [],
      "scanned": [{"name": "foo", "imageName": "foo:latest", "state": "Scanned"}],
      "failed": [], "skipped": [], "stale": [], "fresh": []
    }
  }'
```

## How It Works

1. Watchtower runs on a schedule, checks for updated container images, and POSTs a `json.v1` payload to `localhost:8088/webhook`
2. Pentarou parses the `report.updated` array from the payload
3. For each updated container:
   - **Mapped container**: Looks up the GitHub repo in the built-in map, fetches the latest release notes (tag, body, URL) from the GitHub API, and posts a rich notification with full release notes
   - **Unmapped container**: Posts a minimal notification with just the container name and image
   - **GitHub API failure**: Posts the notification without release notes and logs the error
4. Each container gets its own Matrix message, sent sequentially with a short delay to avoid rate limiting
5. If nothing was updated, posts nothing -- silence is correct

### GitHub release notes

Pentarou fetches release notes from the GitHub API using the `application/vnd.github.full+json` media type, which returns pre-rendered HTML directly from GitHub. This means:

- No local Markdown-to-HTML conversion needed (no extra dependencies)
- GitHub's own rendering handles custom extensions, autolinked references, etc.
- Matrix messages include both a plain text fallback (`body`) and HTML (`formatted_body`) so Element, Cinny, FluffyChat, etc. render headings, bullet points, code blocks, and links correctly

GitHub API calls for multiple containers in the same webhook are made concurrently (up to 4 at a time) to minimize latency.

### Container → GitHub repo map

Pentarou includes a built-in map of container names to GitHub repositories. When a container name appears in this map, Pentarou fetches its latest release notes:

| Container | GitHub Repo |
|---|---|
| `akkoma-akkoma-1` | [akkoma-im/akkoma](https://github.com/akkoma-im/akkoma) |
| `lemmy-lemmy-1` | [LemmyNet/lemmy](https://github.com/LemmyNet/lemmy) |
| `lemmy-lemmy-ui-1` | [LemmyNet/lemmy-ui](https://github.com/LemmyNet/lemmy-ui) |
| `lemmy-pictrs-1` | [asonix/pictrs](https://github.com/asonix/pictrs) |
| `mash-authentik-server` | [goauthentik/authentik](https://github.com/goauthentik/authentik) |
| `mash-miniflux` | [miniflux/miniflux](https://github.com/miniflux/miniflux) |
| `mash-traefik` | [traefik/traefik](https://github.com/traefik/traefik) |
| `mash-uptime-kuma` | [louislam/uptime-kuma](https://github.com/louislam/uptime-kuma) |
| `mash-writefreely` | [writefreely/writefreely](https://github.com/writefreely/writefreely) |

Containers not in this map (databases, proxies, sidecars) still get notified -- they just won't include release notes.

## What Pentarou Says

### Mapped container (with release notes)

```
🔔 Update available: akkoma-akkoma-1
📦 ghcr.io/akkoma-im/akkoma:latest
🏷️ v3.13.0
📝
## Changes
- Fixed a bug
- Added a feature
🔗 https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0
```

### Unmapped container (no release notes)

```
🔔 Update available: akkoma-db-1
📦 postgres:16
```

### Mapped container, GitHub API failure

```
🔔 Update available: mash-traefik
📦 traefik:latest
⚠️ Could not fetch release notes.
```

### No changes

Nothing. Silence. The room is blissfully unbothered.

### Malformed payload

Logged server-side, HTTP 400 returned to Watchtower. Nothing posted to Matrix.

## Matrix Setup

Pentarou uses [mautrix-go](https://github.com/mautrix/go) with full E2EE support.

1. Create a bot user on your homeserver (e.g. `@pentarou:yourdomain.com`)
2. Set `matrix.username` in config and `PENTAROU_MATRIX_PASSWORD` in your environment — Pentarou will login automatically, store the token in `data/device.json`, and reuse it across restarts
3. Invite the bot to the target room and have it join
4. Set the `room_id` in config (the internal ID like `!abc123:yourdomain.com`, not the alias)
5. If using a separate updates room, also set `watchtower_updates_room` and invite the bot there too

On first run with username/password, Pentarou bootstraps cross-signing so the bot's device is automatically trusted by other users — no manual emoji verification needed.

The bot only needs `m.room.message` send permission.

## Project Structure

```
Pentarou/
├── main.go              # Entry point -- loads config, starts server
├── server.go            # HTTP handler for /webhook, orchestrates processing
├── formatter.go         # Watchtower payload parsing, repo map, message formatting
├── github.go            # GitHub releases API client
├── matrix.go            # mautrix-go client with E2EE, Notifier interface
├── auth.go              # Matrix login, token validation, device.json persistence
├── notify.go            # Legacy Matrix posting via net/http, 3x retry with backoff
├── config.go            # YAML config + env var overrides
├── formatter_test.go
├── github_test.go
├── server_test.go
├── notify_test.go
├── config/
│   └── config.example.yml
├── systemd/
│   └── pentarou.service
├── go.mod
└── go.sum
```

## Configuration Reference

| Key | Required | Default | Description |
|---|---|---|---|
| `matrix.homeserver` | Yes | -- | Base URL of your Matrix homeserver |
| `matrix.room_id` | Yes | -- | Internal room ID (fallback for notifications) |
| `matrix.username` | No* | -- | Matrix user ID for password login (e.g. `@pentarou:example.com`) |
| `matrix.password` | No* | -- | Matrix password (prefer `PENTAROU_MATRIX_PASSWORD` env var) |
| `matrix.access_token` | No* | -- | Explicit access token (prefer `PENTAROU_MATRIX_TOKEN` env var) |
| `matrix.bot_displayname` | No | `Pentarou` | Display name for the bot |
| `matrix.encryption` | No | `true` | Enable E2EE via mautrix-go |
| `matrix.device_id` | No | `PENTAROU` | Stable device ID (do not change after first run) |
| `matrix.database_path` | No | `pentarou-crypto.db` | SQLite database for E2EE crypto state |
| `matrix.pickle_key` | No | `pentarou` | Key to encrypt the crypto store at rest |
| `matrix.data_dir` | No | `data` | Directory for device.json and crypto DB |

*Either `access_token` or both `username`+`password` must be provided.
| `webhook.host` | No | `127.0.0.1` | Bind address for the webhook server |
| `webhook.port` | No | `8088` | Port for the webhook server |
| `webhook.token` | No | -- | Shared secret for webhook authentication (prefer `PENTAROU_WEBHOOK_TOKEN` env var) |
| `notifications.skip_if_no_changes` | No | `true` | Post nothing when Watchtower reports no updates |
| `notifications.watchtower_updates_room` | No | (uses `matrix.room_id`) | Dedicated room for update notifications |
| `notifications.github_token` | No | -- | GitHub token for higher API rate limits (prefer `PENTAROU_GITHUB_TOKEN` env var) |

### Environment Variables

| Variable | Overrides | Description |
|---|---|---|
| `PENTAROU_MATRIX_TOKEN` | `matrix.access_token` | Explicit Matrix access token (skips password login) |
| `PENTAROU_MATRIX_USER` | `matrix.username` | Matrix user ID for password login |
| `PENTAROU_MATRIX_PASSWORD` | `matrix.password` | Matrix password for automatic login |
| `PENTAROU_GITHUB_TOKEN` | `notifications.github_token` | GitHub personal access token (optional, raises rate limit from 60 to 5,000 requests/hour) |
| `PENTAROU_WEBHOOK_TOKEN` | `webhook.token` | Shared secret for webhook authentication (recommended when binding to `0.0.0.0`) |

## Running Tests

```bash
go test ./... -v
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `400: payload has no report data` | Watchtower not using `json.v1` template | Set `WATCHTOWER_NOTIFICATION_TEMPLATE=json.v1` in your Watchtower config |
| Config file not found | Config file doesn't exist yet | `cp config/config.example.yml config/config.yml` and edit it |
| `PENTAROU_MATRIX_TOKEN` not taking effect | Env var not exported, or `.env` path wrong in systemd unit | Verify `EnvironmentFile=` in the service unit points to your actual `.env` |
| Matrix post returns 403 | Bot not in the room or lacks permission | Invite the bot and make sure it has send-message permission |
| Matrix post returns 401 | Access token is wrong or expired | Regenerate the token, update config or env var |
| Watchtower not sending webhooks | Notification URL misconfigured | Verify `WATCHTOWER_NOTIFICATION_URL` uses `generic+http://` prefix |
| `⚠️ Could not fetch release notes` | GitHub API rate limited or repo not found | Set `PENTAROU_GITHUB_TOKEN` for higher rate limits; verify repo exists in the map |
| No notification after update | Watchtower reported no changes | `curl` the webhook endpoint manually to test; check Pentarou logs |
| Port 8088 already in use | Another service on that port | Change `webhook.port` in config |

## License

See repository for license details.
