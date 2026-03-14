"""Entry point for Pentarou.

Usage:
    python -m src
    python -m src --config /path/to/config.yml
"""

import argparse
import logging

from src.config import load_config
from src.server import run_server

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)


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
    args = parser.parse_args()

    config = load_config(args.config)

    webhook = config.get("webhook", {})
    host = webhook.get("host", "127.0.0.1")
    port = webhook.get("port", 8088)

    run_server(host, port)


if __name__ == "__main__":
    main()
