from datetime import datetime, timezone


def _display_name(container_name: str, service_names: dict[str, str]) -> str:
    return service_names.get(container_name, container_name)


def _short_digest(digest: str) -> str:
    if digest.startswith("sha256:"):
        return "sha256:" + digest[7:19]
    return digest[:19]


def format_update_summary(diff_result: dict, config: dict) -> tuple[str, str]:
    service_names = config.get("notifications", {}).get("service_names", {})
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")

    lines = ["\U0001f427 **Pentarou -- Update Complete**", ""]

    updated = diff_result.get("updated", [])
    if updated:
        lines.append(f"**Updated ({len(updated)})**")
        for entry in updated:
            name = _display_name(entry["name"], service_names)
            old = _short_digest(entry["old_digest"])
            new = _short_digest(entry["new_digest"])
            lines.append(f"- {name}: `{old}` -> `{new}`")
        lines.append("")

    added = diff_result.get("added", [])
    if added:
        lines.append(f"**Added ({len(added)})**")
        for entry in added:
            name = _display_name(entry["name"], service_names)
            lines.append(f"- {name} (new)")
        lines.append("")

    removed = diff_result.get("removed", [])
    if removed:
        lines.append(f"**Removed ({len(removed)})**")
        for entry in removed:
            name = _display_name(entry["name"], service_names)
            lines.append(f"- {name}")
        lines.append("")

    lines.append(f"Run completed: {now}")

    plain = "\n".join(lines)
    html = _markdown_to_html(plain)
    return plain, html


def format_warning(minutes: int) -> tuple[str, str]:
    plain = (
        f"\U0001f427 Pentarou: Scheduled maintenance starting in {minutes} minutes.\n"
        "Brief service interruptions are possible."
    )
    html = plain.replace("\n", "<br>")
    return plain, html


def format_start() -> tuple[str, str]:
    plain = "\U0001f427 Pentarou: Update run starting now."
    return plain, plain


def format_failure(exit_code: int) -> tuple[str, str]:
    plain = (
        f"\U0001f427 Pentarou: Update run failed (exit code {exit_code}).\n"
        "One or more services may be affected. Manual inspection required."
    )
    html = plain.replace("\n", "<br>")
    return plain, html


def _markdown_to_html(text: str) -> str:
    import re
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
