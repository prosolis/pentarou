import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from threading import Thread
from unittest.mock import patch

from src.notify import post_message


class _MatrixHandler(BaseHTTPRequestHandler):
    events_received = []

    def do_PUT(self):
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length)) if length else {}
        _MatrixHandler.events_received.append({
            "path": self.path,
            "auth": self.headers.get("Authorization"),
            "body": body,
        })
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"event_id": "$test123"}).encode())

    def log_message(self, *args):
        pass


def test_post_message_sends_correctly():
    _MatrixHandler.events_received = []
    server = HTTPServer(("127.0.0.1", 0), _MatrixHandler)
    port = server.server_address[1]
    thread = Thread(target=server.handle_request, daemon=True)
    thread.start()

    result = post_message(
        homeserver=f"http://127.0.0.1:{port}",
        room_id="!test:example.com",
        access_token="syt_testtoken",
        plain_body="Hello",
        html_body="<p>Hello</p>",
    )

    thread.join(timeout=5)
    server.server_close()

    assert result is True
    assert len(_MatrixHandler.events_received) == 1
    event = _MatrixHandler.events_received[0]
    assert "/_matrix/client/v3/rooms/" in event["path"]
    assert event["auth"] == "Bearer syt_testtoken"
    assert event["body"]["msgtype"] == "m.text"
    assert event["body"]["body"] == "Hello"
    assert event["body"]["formatted_body"] == "<p>Hello</p>"


def test_post_message_plain_only():
    _MatrixHandler.events_received = []
    server = HTTPServer(("127.0.0.1", 0), _MatrixHandler)
    port = server.server_address[1]
    thread = Thread(target=server.handle_request, daemon=True)
    thread.start()

    result = post_message(
        homeserver=f"http://127.0.0.1:{port}",
        room_id="!test:example.com",
        access_token="token",
        plain_body="Plain message",
    )

    thread.join(timeout=5)
    server.server_close()

    assert result is True
    body = _MatrixHandler.events_received[0]["body"]
    assert "formatted_body" not in body
    assert "format" not in body


@patch("src.notify.MAX_RETRIES", 1)
@patch("src.notify.BACKOFF_BASE", 0.01)
def test_post_message_retries_on_failure():
    result = post_message(
        homeserver="http://127.0.0.1:1",
        room_id="!test:example.com",
        access_token="token",
        plain_body="Should fail",
    )
    assert result is False
