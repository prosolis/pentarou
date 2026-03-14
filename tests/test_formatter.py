from src.formatter import format_update_report, parse_watchtower_message


_CONFIG = {
    "notifications": {
        "service_names": {
            "akkoma": "Akkoma",
            "postgres": "PostgreSQL",
            "pixelfed": "PixelFed",
        },
    },
}


def test_parse_basic_update():
    msg = "Updating /mash-akkoma (sha256:aaabbb111222 to sha256:cccddd333444)"
    updates = parse_watchtower_message(msg)
    assert len(updates) == 1
    assert updates[0]["name"] == "akkoma"
    assert updates[0]["old_digest"] == "sha256:aaabbb111222"
    assert updates[0]["new_digest"] == "sha256:cccddd333444"


def test_parse_with_container_keyword():
    msg = "Updating container /mash-pixelfed (sha256:aaa to sha256:bbb)"
    updates = parse_watchtower_message(msg)
    assert len(updates) == 1
    assert updates[0]["name"] == "pixelfed"


def test_parse_multiple_updates():
    msg = (
        "Updating /mash-akkoma (sha256:aaa to sha256:bbb)\n"
        "Updating /mash-postgres (sha256:ccc to sha256:ddd)\n"
        "Updating /mash-pixelfed (sha256:eee to sha256:fff)"
    )
    updates = parse_watchtower_message(msg)
    assert len(updates) == 3
    names = [u["name"] for u in updates]
    assert "akkoma" in names
    assert "postgres" in names
    assert "pixelfed" in names


def test_parse_no_updates():
    msg = "No containers need updating"
    updates = parse_watchtower_message(msg)
    assert updates == []


def test_parse_updated_past_tense():
    msg = "Updated /mash-akkoma (sha256:aaa to sha256:bbb)"
    updates = parse_watchtower_message(msg)
    assert len(updates) == 1


def test_format_report_with_updates():
    msg = (
        "Updating /mash-akkoma (sha256:aaabbbcccddd111222333444 to sha256:777888999000aaabbbcccddd)\n"
        "Updating /mash-postgres (sha256:111222333444555666777888 to sha256:aaabbbcccddd111222333444)"
    )
    result = format_update_report(msg, _CONFIG)
    assert result is not None
    plain, html = result
    assert "Pentarou" in plain
    assert "Akkoma" in plain
    assert "PostgreSQL" in plain
    assert "sha256:aaabbbcccddd" in plain
    assert "\u2192" in plain


def test_format_report_no_updates_returns_none():
    msg = "No containers need updating"
    result = format_update_report(msg, _CONFIG)
    assert result is None


def test_format_report_uses_service_names():
    msg = "Updating /mash-postgres (sha256:aaa to sha256:bbb)"
    result = format_update_report(msg, _CONFIG)
    assert result is not None
    plain, _ = result
    assert "PostgreSQL" in plain


def test_format_report_html_has_tags():
    msg = "Updating /mash-akkoma (sha256:aaa to sha256:bbb)"
    result = format_update_report(msg, _CONFIG)
    assert result is not None
    _, html = result
    assert "<strong>" in html
    assert "<li>" in html


def test_format_report_unknown_service_uses_raw_name():
    msg = "Updating /mash-somethingelse (sha256:aaa to sha256:bbb)"
    result = format_update_report(msg, _CONFIG)
    assert result is not None
    plain, _ = result
    assert "somethingelse" in plain


def test_parse_no_mash_prefix():
    msg = "Updating /mycontainer (sha256:aaa to sha256:bbb)"
    updates = parse_watchtower_message(msg)
    assert len(updates) == 1
    assert updates[0]["name"] == "mycontainer"
