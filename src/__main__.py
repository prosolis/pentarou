"""CLI entry point for Pentarou.

Usage:
    python -m src warn              Post T-5 warning to Matrix
    python -m src update            Run full update cycle
    python -m src update --dry-run  Snapshot only, no update or Matrix post
"""

import argparse
import logging
import subprocess
import sys

from src.config import get_config, load_config
from src.diff import compute_diff, has_changes
from src.formatter import (
    format_failure,
    format_start,
    format_update_summary,
    format_warning,
)
from src.notify import send
from src.snapshot import take_snapshot

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
log = logging.getLogger("pentarou")


def cmd_warn(config: dict) -> int:
    minutes = config.get("schedule", {}).get("warn_minutes_before", 5)
    plain, html = format_warning(minutes)
    log.info("Sending %d-minute warning", minutes)
    send(config, plain, html)
    return 0


def cmd_update(config: dict, dry_run: bool = False) -> int:
    if not dry_run:
        plain, _ = format_start()
        send(config, plain)

    log.info("Taking pre-update snapshot")
    before = take_snapshot()

    if dry_run:
        log.info("Dry run -- skipping update and notification")
        log.info("Snapshot: %d containers", len(before.get("containers", {})))
        for name, info in sorted(before.get("containers", {}).items()):
            log.info("  %s: %s", name, info["digest"][:19])
        return 0

    mash = config.get("mash", {})
    playbook_dir = mash.get("playbook_dir", ".")
    just_bin = mash.get("just_bin", "just")

    log.info("Running %s install-all in %s", just_bin, playbook_dir)
    result = subprocess.run(
        [just_bin, "install-all"],
        cwd=playbook_dir,
        capture_output=True, text=True,
    )

    if result.returncode != 0:
        log.error("Update failed (exit code %d)", result.returncode)
        if result.stderr:
            log.error("stderr: %s", result.stderr[-500:])
        plain, html = format_failure(result.returncode)
        send(config, plain, html)
        return result.returncode

    log.info("Taking post-update snapshot")
    after = take_snapshot()

    diff_result = compute_diff(before, after)

    skip_no_changes = config.get("notifications", {}).get("skip_if_no_changes", True)
    if not has_changes(diff_result):
        if skip_no_changes:
            log.info("No changes detected -- nothing to announce")
            return 0
        log.info("No changes detected (posting anyway per config)")

    plain, html = format_update_summary(diff_result, config)
    log.info("Posting update summary to Matrix")
    send(config, plain, html)
    return 0


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="pentarou",
        description="Pentarou -- infrastructure update announcer for Matrix",
    )
    parser.add_argument(
        "--config", "-c",
        default=None,
        help="path to config.yml (default: config/config.yml)",
    )

    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("warn", help="post T-5 warning to Matrix")

    update_parser = sub.add_parser("update", help="run full update cycle")
    update_parser.add_argument(
        "--dry-run",
        action="store_true",
        help="snapshot only, no update or Matrix post",
    )

    args = parser.parse_args()

    config = load_config(args.config)

    if args.command == "warn":
        sys.exit(cmd_warn(config))
    elif args.command == "update":
        sys.exit(cmd_update(config, dry_run=args.dry_run))


if __name__ == "__main__":
    main()
