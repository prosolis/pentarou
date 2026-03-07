import json
import logging
import subprocess
from datetime import datetime, timezone

log = logging.getLogger(__name__)


def take_snapshot() -> dict:
    result = subprocess.run(
        ["docker", "ps", "-q", "--no-trunc"],
        capture_output=True, text=True, check=True,
    )
    ids = [line for line in result.stdout.strip().splitlines() if line]

    if not ids:
        log.warning("No running containers found")
        return {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "containers": {},
        }

    result = subprocess.run(
        ["docker", "inspect"] + ids,
        capture_output=True, text=True, check=True,
    )
    inspected = json.loads(result.stdout)

    containers: dict[str, dict] = {}
    for c in inspected:
        name = c["Name"].lstrip("/")
        containers[name] = {
            "image": c["Config"]["Image"],
            "digest": c["Image"],
            "created": c["Created"],
        }

    log.info("Snapshot captured: %d containers", len(containers))
    return {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "containers": containers,
    }
