import json
import logging
from http.server import BaseHTTPRequestHandler, HTTPServer

from src.config import get_config
from src.formatter import format_update_report
from src.notify import send

log = logging.getLogger(__name__)


class WebhookHandler(BaseHTTPRequestHandler):

    def do_POST(self):
        if self.path != "/webhook":
            self.send_response(404)
            self.end_headers()
            return

        content_length = int(self.headers.get("Content-Length", 0))
        if content_length == 0:
            self._error(400, "Empty request body")
            return

        try:
            raw = self.rfile.read(content_length)
            payload = json.loads(raw)
        except (json.JSONDecodeError, ValueError) as e:
            self._error(400, f"Invalid JSON: {e}")
            return

        message = payload.get("message")
        if not message or not isinstance(message, str):
            self._error(400, "Missing or invalid 'message' field")
            return

        log.info("Received webhook: title=%s level=%s",
                 payload.get("title", "?"), payload.get("level", "?"))

        config = get_config()
        skip_no_changes = config.get("notifications", {}).get(
            "skip_if_no_changes", True)

        result = format_update_report(message, config)
        if result is None:
            if skip_no_changes:
                log.info("No updates in payload -- skipping notification")
            else:
                log.info("No updates in payload (posting anyway per config)")
                plain = "\U0001f427 **Pentarou \u2014 Update Report**\n\nAll containers are up to date."
                html = "<p>\U0001f427 <strong>Pentarou \u2014 Update Report</strong></p><p>All containers are up to date.</p>"
                send(config, plain, html)
            self.send_response(200)
            self.end_headers()
            return

        plain, html = result
        send(config, plain, html)

        self.send_response(200)
        self.end_headers()

    def _error(self, code: int, reason: str):
        log.warning("Webhook error %d: %s", code, reason)
        self.send_response(code)
        self.end_headers()

    def log_message(self, format, *args):
        log.debug("HTTP: %s", format % args)


def run_server(host: str, port: int):
    server = HTTPServer((host, port), WebhookHandler)
    log.info("Pentarou listening on %s:%d", host, port)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log.info("Shutting down")
    finally:
        server.server_close()
