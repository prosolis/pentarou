import json
from http.server import HTTPServer
from threading import Thread
from urllib.request import Request, urlopen

from src.config import load_config
from src.server import WebhookHandler


def _start_server():
    server = HTTPServer(("127.0.0.1", 0), WebhookHandler)
    port = server.server_address[1]
    thread = Thread(target=server.handle_request, daemon=True)
    thread.start()
    return server, port, thread


def _post(port, path, body):
    data = json.dumps(body).encode()
    req = Request(
        f"http://127.0.0.1:{port}{path}",
        data=data,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    return urlopen(req, timeout=5)


def _setup_config(tmp_path):
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "matrix:\n"
        "  homeserver: 'http://127.0.0.1:1'\n"
        "  room_id: '!test:example.com'\n"
        "  access_token: 'testtoken'\n"
        "webhook:\n"
        "  host: '127.0.0.1'\n"
        "  port: 0\n"
        "notifications:\n"
        "  skip_if_no_changes: true\n"
    )
    load_config(str(config_file))


def test_webhook_returns_200_on_valid_payload(tmp_path):
    _setup_config(tmp_path)
    server, port, thread = _start_server()

    payload = {
        "title": "Watchtower updates",
        "message": "No containers need updating",
        "level": "info",
    }
    resp = _post(port, "/webhook", payload)
    assert resp.status == 200

    thread.join(timeout=5)
    server.server_close()


def test_webhook_returns_404_on_wrong_path(tmp_path):
    _setup_config(tmp_path)
    server, port, thread = _start_server()

    try:
        _post(port, "/wrong", {"message": "test"})
        assert False, "Should have raised"
    except Exception as e:
        assert "404" in str(e)

    thread.join(timeout=5)
    server.server_close()


def test_webhook_returns_400_on_missing_message(tmp_path):
    _setup_config(tmp_path)
    server, port, thread = _start_server()

    try:
        _post(port, "/webhook", {"title": "test"})
        assert False, "Should have raised"
    except Exception as e:
        assert "400" in str(e)

    thread.join(timeout=5)
    server.server_close()


def test_webhook_returns_400_on_invalid_json(tmp_path):
    _setup_config(tmp_path)
    server, port, thread = _start_server()

    req = Request(
        f"http://127.0.0.1:{port}/webhook",
        data=b"not json",
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        urlopen(req, timeout=5)
        assert False, "Should have raised"
    except Exception as e:
        assert "400" in str(e)

    thread.join(timeout=5)
    server.server_close()
