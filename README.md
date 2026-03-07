# 🐧 Pentarou

Infrastructure update announcer for a mash-playbook fediverse stack. Posts container
update summaries to a Matrix room after each `just install-all` run.

Part of the TwinBee/Parodius community bot ecosystem. Named after Pentarou the penguin
from the Parodius franchise (Konami).

## Siblings
- **Bellhop** — media request portal
- **GwinBee** — gaming deals aggregator
- **Melora** — webhook receiver / *arr stack notifications

## Setup

```bash
cp config/config.example.yml config/config.yml
# edit config/config.yml with your Matrix details and mash-playbook path

pip install -r requirements.txt
```

## Usage

```bash
# Run update with notifications
./scripts/run-update.sh

# Dry run (no update, no Matrix post)
./scripts/run-update.sh --dry-run

# Test Matrix connectivity
PENTAROU_MATRIX_TOKEN=your_token python src/notify.py \
  --homeserver https://matrix.yourdomain.com \
  --room-id '!roomid:yourdomain.com'
```

## How It Works

1. Snapshots running container image digests before the update
2. Runs `just install-all` in the configured mash-playbook directory
3. Snapshots again after the update
4. Diffs the two snapshots to find what changed
5. Posts a formatted summary to the configured Matrix room
6. Posts nothing if no containers changed

## Requirements
- Python 3.11+
- Docker (for container inspection)
- `just` (mash-playbook command runner)
- A Matrix bot user with access to the target room
