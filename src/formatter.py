import re
from datetime import datetime, timezone


def _display_name(container_name: str, service_names: dict[str, str]) -> str:
    return service_names.get(container_name, container_name)


def _short_digest(digest: str) -> str:
    if digest.startswith("sha256:"):
        return "sha256:" + digest[7:19]
    return digest[:19]


def _clean_container_name(raw: str) -> str:
    """Strip leading / and common prefixes like mash-."""
    name = raw.lstrip("/")
    if name.startswith("mash-"):
        name = name[5:]
    return name


def parse_watchtower_message(message: str) -> list[dict]:
    """Parse Watchtower's report message into a list of updated containers.

    Handles common Watchtower report formats:
      - "Updating /mash-akkoma (sha256:aaa to sha256:bbb)"
      - "Updating container /mash-akkoma (sha256:aaa to sha256:bbb)"
    """
    updates = []

    pattern = re.compile(
        r"[Uu]pdat(?:ing|ed)\s+(?:container\s+)?"
        r"(/?\S+)"
        r"\s+\((\S+)\s+to\s+(\S+)\)"
    )

    for match in pattern.finditer(message):
        raw_name, old_digest, new_digest = match.groups()
        updates.append({
            "name": _clean_container_name(raw_name),
            "old_digest": old_digest,
            "new_digest": new_digest,
        })

    return updates


def format_update_report(
    message: str, config: dict,
) -> tuple[str, str] | None:
    """Format a Watchtower message into a Matrix notification.

    Returns (plain, html) or None if there are no updates to report.
    """
    updates = parse_watchtower_message(message)
    if not updates:
        return None

    service_names = config.get("notifications", {}).get("service_names", {})
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")

    lines = ["\U0001f427 **Pentarou \u2014 Update Report**", ""]

    for entry in updates:
        name = _display_name(entry["name"], service_names)
        old = _short_digest(entry["old_digest"])
        new = _short_digest(entry["new_digest"])
        lines.append(f"- {name}: `{old}` \u2192 `{new}`")

    lines.append("")
    lines.append(now)

    plain = "\n".join(lines)
    html = _markdown_to_html(plain)
    return plain, html


def _markdown_to_html(text: str) -> str:
    html = text
    html = re.sub(r'`([^`]+)`', r'<code>\1</code>', html)
    html = re.sub(r'\*\*([^*]+)\*\*', r'<strong>\1</strong>', html)
    lines = html.split("\n")
    result = []
    in_list = False
    for line in lines:
        if line.startswith("- "):
            if not in_list:
                result.append("<ul>")
                in_list = True
            result.append(f"<li>{line[2:]}</li>")
        else:
            if in_list:
                result.append("</ul>")
                in_list = False
            if line.strip():
                result.append(f"<p>{line}</p>")
    if in_list:
        result.append("</ul>")
    return "\n".join(result)
