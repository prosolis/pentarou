import json
import logging
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

log = logging.getLogger(__name__)

MAX_RETRIES = 3
BACKOFF_BASE = 2


def post_message(
    homeserver: str,
    room_id: str,
    access_token: str,
    plain_body: str,
    html_body: str | None = None,
) -> bool:
    txn_id = str(uuid.uuid4())
    encoded_room = urllib.parse.quote(room_id, safe="")
    url = (
        f"{homeserver.rstrip('/')}/_matrix/client/v3/rooms/"
        f"{encoded_room}/send/m.room.message/{txn_id}"
    )

    content: dict = {"msgtype": "m.text", "body": plain_body}
    if html_body:
        content["format"] = "org.matrix.custom.html"
        content["formatted_body"] = html_body

    data = json.dumps(content).encode()
    req = urllib.request.Request(url, data=data, method="PUT")
    req.add_header("Authorization", f"Bearer {access_token}")
    req.add_header("Content-Type", "application/json")

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                resp_data = json.loads(resp.read())
                log.info("Message sent: %s", resp_data.get("event_id", "?"))
                return True
        except (urllib.error.URLError, urllib.error.HTTPError, OSError) as e:
            if attempt < MAX_RETRIES:
                wait = BACKOFF_BASE ** attempt
                log.warning(
                    "Matrix post failed (attempt %d/%d), retrying in %ds: %s",
                    attempt, MAX_RETRIES, wait, e,
                )
                time.sleep(wait)
            else:
                log.error("Matrix post failed after %d attempts: %s", MAX_RETRIES, e)
    return False


def send(config: dict, plain_body: str, html_body: str | None = None) -> bool:
    matrix = config["matrix"]
    return post_message(
        homeserver=matrix["homeserver"],
        room_id=matrix["room_id"],
        access_token=matrix["access_token"],
        plain_body=plain_body,
        html_body=html_body,
    )
