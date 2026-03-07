import logging
import os
from pathlib import Path

import yaml

log = logging.getLogger(__name__)

_config: dict | None = None

_DEFAULT_PATH = Path(__file__).resolve().parent.parent / "config" / "config.yml"


def load_config(path: str | Path | None = None) -> dict:
    global _config
    config_path = Path(path) if path else _DEFAULT_PATH

    with open(config_path) as f:
        _config = yaml.safe_load(f)

    env_token = os.environ.get("PENTAROU_MATRIX_TOKEN")
    if env_token:
        _config["matrix"]["access_token"] = env_token
        log.info("Matrix token overridden from PENTAROU_MATRIX_TOKEN env var")

    return _config


def get_config() -> dict:
    if _config is None:
        raise RuntimeError("Config not loaded -- call load_config() first")
    return _config
