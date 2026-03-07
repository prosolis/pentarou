# Pentarou

A diligent penguin that watches over your fediverse stack so you don't have to.

Pentarou monitors a [mash-playbook](https://github.com/mother-of-all-self-hosting/mash-playbook)/Docker-based fediverse stack, runs `just install-all` on a schedule, and tells your Matrix room what changed. If nothing changed, he keeps quiet. If something broke, he screams. Good penguin.

Named after Pentarou the penguin from the Parodius franchise (Konami) -- the one who shows up uninvited, causes a scene, and somehow makes everything better. Part of the TwinBee/Parodius community bot ecosystem.

## Siblings

- **Bellhop** -- media request portal
- **GwinBee** -- gaming deals aggregator (bell-collecting optional)
- **Melora** -- webhook receiver / *arr stack notifications

## Requirements

- Python 3.11+
- Docker (Pentarou needs to peek at your containers)
- `just` (mash-playbook command runner)
- A Matrix bot user with message-send permission in the target room
- systemd (Pentarou runs on a schedule, not vibes)

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
  # Override with PENTAROU_MATRIX_TOKEN env var -- never hardcode in production
  access_token: "YOUR_ACCESS_TOKEN_HERE"
  bot_displayname: "Pentarou"

mash:
  playbook_dir: "/path/to/mash-playbook"
  just_bin: "just"  # override if not in PATH

schedule:
  # UTC 06:00 -- the globally least-bad window for distributed communities:
  # Seattle ~10pm | Lisbon 6am | London 6am | Tokyo 3pm | Sydney 4pm
  update_time: "06:00"
  timezone: "UTC"
  warn_minutes_before: 5

notifications:
  skip_if_no_changes: true
  include_supporting_services: true

  # Human-friendly display names for containers
  # Falls back to the raw container name if not listed
  service_names:
    akkoma: "Akkoma"
    pixelfed: "PixelFed"
    lemmy: "Lemmy"
    lemmy-ui: "Lemmy UI"
    writefreely: "WriteFreely"
    funkwhale: "Funkwhale"
    funkwhale-api: "Funkwhale API"
    funkwhale-beat: "Funkwhale Beat"
    postgres: "PostgreSQL"
    redis: "Redis"
    traefik: "Traefik"
    ddclient: "ddclient"
```

### 2. Set up a virtual environment

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

### 3. Set the Matrix token

The access token can live in `config.yml` for local testing, but in production, keep it out of version control. Pentarou isn't going to power-up your secrets for you:

```bash
export PENTAROU_MATRIX_TOKEN="syt_your_real_token_here"
```

For systemd, put it in `/opt/pentarou/.env`:

```bash
PENTAROU_MATRIX_TOKEN=syt_your_real_token_here
```

The env var always wins over whatever is in the config file.

### 4. Install systemd timers

Copy the unit files and adjust paths if you installed somewhere other than `/opt/pentarou`:

```bash
sudo cp systemd/pentarou-warn.service /etc/systemd/system/
sudo cp systemd/pentarou-warn.timer   /etc/systemd/system/
sudo cp systemd/pentarou-update.service /etc/systemd/system/
sudo cp systemd/pentarou-update.timer   /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now pentarou-warn.timer
sudo systemctl enable --now pentarou-update.timer
```

If your update window is not 06:00 UTC, edit the `OnCalendar` values in both timer units. The warn timer should fire `warn_minutes_before` minutes ahead of the update timer. Math is left as an exercise for the reader.

## Usage

### Manual run

```bash
# Full update cycle: snapshot -> install-all -> snapshot -> diff -> notify
python -m src update

# Dry run -- snapshots containers but doesn't touch anything or post to Matrix
# Great for "what would happen if I ran this right now?"
python -m src update --dry-run

# Post the T-5 warning manually (useful for testing Matrix connectivity)
python -m src warn

# Use an alternate config file
python -m src -c /etc/pentarou/config.yml update
```

Or use the convenience wrapper:

```bash
./scripts/run-update.sh
./scripts/run-update.sh --dry-run
```

### Scheduled run (systemd)

Two timer/service pairs handle the lifecycle. Set them up once, then go collect bells or whatever it is you do when you're not administering servers:

| Timer | Fires | What the penguin does |
|---|---|---|
| `pentarou-warn.timer` | 05:55 UTC | Posts a 5-minute heads-up to Matrix |
| `pentarou-update.timer` | 06:00 UTC | Runs the full update cycle |

Check on your penguin:

```bash
systemctl list-timers pentarou-*
journalctl -u pentarou-update.service -n 50
journalctl -u pentarou-warn.service -n 20
```

## How It Works

Like a Parodius stage -- methodical, colorful, and occasionally explosive:

1. Posts "Update run starting now" to the Matrix room
2. Snapshots all running container image digests via `docker inspect`
3. Runs `just install-all` in the configured mash-playbook directory
4. Snapshots container digests again
5. Diffs the two snapshots to find updated, added, and removed containers
6. Posts a formatted summary to Matrix -- or stays silent if nothing changed
7. If `just install-all` exits non-zero, posts a failure notice instead

Pentarou never raises exceptions during notification. Matrix is down? He logs it and waddles on. Three retries with exponential backoff, then silence. The update must flow -- no notification failure should ever block your infrastructure.

## What Pentarou Says

### T-5 warning

```
Pentarou: Scheduled maintenance starting in 5 minutes.
Brief service interruptions are possible.
```

### Update complete (with changes)

```
Pentarou -- Update Complete

Updated (3)
- Akkoma: sha256:aaabbbcccddd -> sha256:111222333444
- PixelFed: sha256:555666777888 -> sha256:999000aaabbb
- PostgreSQL: sha256:cccdddeeefff -> sha256:000111222333

Added (1)
- WriteFreely (new)

Run completed: 2025-03-06 06:04 UTC
```

### No changes

Nothing. Silence. The room is blissfully unbothered. Like a Moai head between stages -- just vibing.

### Something broke

```
Pentarou: Update run failed (exit code 1).
One or more services may be affected. Manual inspection required.
```

This is the "a bell just hit you" message. Pentarou tells you it broke but won't pretend to know why -- that's your job.

## Matrix Setup

Pentarou talks to Matrix directly via the Client-Server API using Python's stdlib `urllib`. No SDK, no extra dependencies beyond what ships with Python. He travels light -- one penguin, one `urllib`, no continues.

1. Create a bot user on your homeserver (e.g. `@pentarou:yourdomain.com`)
2. Generate an access token for the bot
3. Invite the bot to the target room and have it join
4. Set the `room_id` in config (this is the internal ID like `!abc123:yourdomain.com`, not the human-readable alias)

The bot only needs `m.room.message` send permission. It doesn't read messages, manage rooms, or do anything else. Unlike its namesake, this Pentarou minds his own business.

## Project Structure

```
Pentarou/
├── src/
│   ├── __main__.py      # CLI entry point (warn / update / --dry-run)
│   ├── config.py         # YAML config + PENTAROU_MATRIX_TOKEN env override
│   ├── snapshot.py        # Container digest capture via docker inspect
│   ├── diff.py            # Before/after snapshot comparison
│   ├── formatter.py       # Matrix message formatting (plain + HTML)
│   └── notify.py          # Matrix posting via urllib, 3x retry with backoff
├── config/
│   └── config.example.yml
├── systemd/
│   ├── pentarou-warn.service
│   ├── pentarou-warn.timer
│   ├── pentarou-update.service
│   └── pentarou-update.timer
├── tests/
│   ├── test_diff.py
│   ├── test_formatter.py
│   └── test_notify.py
├── scripts/
│   └── run-update.sh
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
| `mash.playbook_dir` | Yes | -- | Absolute path to the mash-playbook directory |
| `mash.just_bin` | No | `just` | Path to `just` binary if not in PATH |
| `schedule.update_time` | No | `06:00` | Daily update time (reference only; actual schedule lives in systemd timers) |
| `schedule.timezone` | No | `UTC` | Timezone for the update window |
| `schedule.warn_minutes_before` | No | `5` | Minutes before update to post warning |
| `notifications.skip_if_no_changes` | No | `true` | Post nothing when no containers changed |
| `notifications.include_supporting_services` | No | `true` | Include supporting services (Postgres, Redis, etc.) in diffs |
| `notifications.service_names` | No | `{}` | Map of container name to human-friendly display name |

## Running Tests

```bash
source .venv/bin/activate
python -m pytest tests/ -v
```

16 tests covering diff logic, message formatting, and Matrix posting (with a real local HTTP server standing in for your homeserver, because mocking is for cowards).

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `FileNotFoundError: config/config.yml` | Config file doesn't exist yet | `cp config/config.example.yml config/config.yml` and edit it |
| `PENTAROU_MATRIX_TOKEN` not taking effect | Env var not exported, or `.env` path wrong in systemd unit | Verify `EnvironmentFile=` in the service unit points to your actual `.env` |
| Matrix post returns 403 | Bot not in the room or lacks permission | Invite the bot and make sure it has send-message permission |
| Matrix post returns 401 | Access token is wrong or expired | Regenerate the token, update config or env var |
| `docker: command not found` | Docker not in PATH under systemd | Add `Environment=PATH=...` to the service unit |
| `just: command not found` | Same deal, different binary | Set `mash.just_bin` to the absolute path (e.g. `/usr/local/bin/just`) |
| Update exits non-zero but services seem fine | Playbook returns non-zero for warnings | Check `journalctl -u pentarou-update.service` for stderr |
| No notification after a real update | Digests didn't actually change | Playbook may have run without pulling new images; use `--dry-run` to check |
| Timer not firing | Timer not enabled | `sudo systemctl enable --now pentarou-update.timer` |
| Snapshots show 0 containers | Docker daemon not running or no containers up | Run `docker ps` as the same user the service runs as |
| Everything works but it's 3am and you're reading this | You're not trusting the penguin enough | Go to bed. Pentarou has the watch |

## License

See repository for license details.
