# Pentarou — Infrastructure Update Announcer
## Build Plan for Claude Code

## What This Is
Pentarou is a bot that monitors a mash-playbook/Docker-based fediverse stack and announces
infrastructure updates to a Matrix room. Runs on a scheduled basis via systemd timers.
Named after Pentarou the penguin from the Parodius franchise (Konami).

## What It Does
1. Warns the Matrix room 5 minutes before an update run begins (generic warning)
2. Announces when the update run starts
3. Snapshots container image digests before the update
4. Runs `just install-all` in the mash-playbook directory
5. Snapshots container image digests after the update
6. Diffs before/after to identify what changed
7. Posts a formatted update summary to the Matrix room
8. Posts a failure notification if the update run exits non-zero
9. Posts nothing if no containers changed

## Stack Context
- Runs on a Debian 12 host managed by mash-playbook
- The mash-playbook stack on this host runs: Akkoma, PixelFed, Lemmy, Lemmy UI,
  WriteFreely, Funkwhale, Funkwhale API, Funkwhale Beat, plus supporting services
  (Postgres — multiple instances, Redis, Traefik, ddclient)
- Matrix/Synapse lives on a SEPARATE host — Pentarou posts TO that Matrix instance,
  it does not run alongside it
- Python 3.11+, minimal dependencies

## Architecture
```
src/
  snapshot.py       # captures container image digests via docker inspect
  diff.py           # compares two snapshots, produces structured change list
  formatter.py      # turns structured diff into human-readable Matrix markdown
  notify.py         # posts messages to Matrix room via Client-Server API, with retry logic
  config.py         # loads and validates config.yml, handles env var overrides
systemd/
  pentarou-warn.service     # posts T-5 warning to Matrix
  pentarou-warn.timer       # fires 5 minutes before update timer
  pentarou-update.service   # runs snapshot → update → snapshot → diff → notify
  pentarou-update.timer     # fires at configured update window (default UTC 06:00)
config/
  config.example.yml
tests/
  test_diff.py
  test_formatter.py
  test_notify.py
```

## Notification Lifecycle

### T-5 minutes (pentarou-warn.timer)
```
🐧 Pentarou: Scheduled maintenance starting in 5 minutes.
Brief service interruptions are possible.
```

### T-0 (pentarou-update.service start)
```
🐧 Pentarou: Update run starting now.
```

### Post-run success with changes
```
🐧 Pentarou — Update Complete

Updated (3)
- Akkoma: sha256:aaabbb → sha256:cccddd
- PixelFed: sha256:111222 → sha256:333444
- PostgreSQL: sha256:aaabbb → sha256:cccddd

Added (1)
- WriteFreely (new)

Run completed: 2025-03-06 06:04 UTC
```

### Post-run success, no changes
- Post nothing. Silence is correct behavior when nothing changed.

### Post-run failure
```
🐧 Pentarou: Update run failed (exit code 1).
One or more services may be affected. Manual inspection required.
```

## Matrix Integration
- Uses Matrix Client-Server API directly via stdlib urllib — no SDK
- Posts as a dedicated bot user (@pentarou:yourdomain.com)
- Messages use Matrix markdown formatting (formatted_body)
- Bot user requires an access token with send message permission in the target room

## Retry Logic
- notify.py retries failed Matrix posts up to 3 times with exponential backoff
- Retry failures are logged but never raise — Pentarou must never block or fail
  the update run itself
- Retry behavior is consistent across all notification types (warn, start, success, failure)

## Update Window
- Scheduled via systemd timers, not cron
- Default: UTC 06:00 — the globally least-bad window for a geographically distributed community
  (Seattle ~10pm, Lisbon 6am, London 6am, Tokyo 3pm, Sydney 4pm)
- Update time and timezone are configurable in config.yml
- Warn timer fires (warn_minutes_before) minutes before the update timer
- systemd timer OnCalendar values are generated from config at install time

## Configuration (config/config.example.yml)
```yaml
matrix:
  homeserver: "https://matrix.yourdomain.com"
  room_id: "!roomid:yourdomain.com"
  # Override with PENTAROU_MATRIX_TOKEN env var — never hardcode in production
  access_token: "YOUR_ACCESS_TOKEN_HERE"
  bot_displayname: "Pentarou"

mash:
  playbook_dir: "/path/to/mash-playbook"
  just_bin: "just"  # override if not in PATH

schedule:
  # UTC 06:00 is the globally least-bad window for distributed communities
  # Seattle: ~10pm | Lisbon: 6am | London: 6am | Tokyo: 3pm | Sydney: 4pm
  update_time: "06:00"
  timezone: "UTC"
  warn_minutes_before: 5

notifications:
  skip_if_no_changes: true
  include_supporting_services: true

  # Optional: human-friendly display names for containers
  # Falls back to container name if not listed
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

## Snapshot Format
```json
{
  "timestamp": "2025-03-06T06:00:00Z",
  "containers": {
    "akkoma": {
      "image": "ghcr.io/someone/akkoma:latest",
      "digest": "sha256:abc123...",
      "created": "2025-03-01T00:00:00Z"
    }
  }
}
```

## Diff Format
```json
{
  "updated": [
    {
      "name": "akkoma",
      "old_digest": "sha256:abc123...",
      "new_digest": "sha256:def456...",
      "old_created": "2025-02-01T00:00:00Z",
      "new_created": "2025-03-05T00:00:00Z",
      "changelog_summary": null
    }
  ],
  "added": [...],
  "removed": [...],
  "unchanged": [...]
}
```

Note: `changelog_summary` is always `null` in the current implementation.
It is included in the structure to support a planned future feature where an LLM
summarizes upstream release notes per updated service. Do not implement this now —
just ensure the field exists in the data structure so it can be populated later
without a schema change.

## Out of Scope
- Pentarou does NOT detect crashed or absent containers — that is a monitoring/alerting
  concern for a separate tool (Uptime Kuma, Healthchecks.io, etc.)
- Pentarou does NOT manage or trigger updates autonomously beyond the scheduled run
- Pentarou does NOT monitor the Matrix/Synapse host (different box, outside scope)
- Pentarou does NOT track OS-level package updates (scope is mash-playbook containers only)
- Changelog summarization via LLM API (planned future feature, not MVP)

## Naming Convention — Bot Ecosystem
- **Bellhop** — media request portal
- **GwinBee** — gaming deals aggregator
- **Melora** — webhook receiver / *arr stack notifications with Matrix threading
- **Pentarou** — this bot (infrastructure update announcements)

All names drawn from the TwinBee and Parodius franchises (Konami).

## Dependencies
- pyyaml — config loading
- pytest — tests
- Everything else: Python stdlib only (urllib for HTTP, subprocess for Docker, json, datetime)
