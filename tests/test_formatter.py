from src.formatter import (
    format_failure,
    format_start,
    format_update_summary,
    format_warning,
)

_CONFIG = {
    "notifications": {
        "service_names": {
            "akkoma": "Akkoma",
            "postgres": "PostgreSQL",
        },
    },
}


def test_format_warning():
    plain, html = format_warning(5)
    assert "5 minutes" in plain
    assert "Pentarou" in plain


def test_format_start():
    plain, html = format_start()
    assert "starting now" in plain


def test_format_failure():
    plain, html = format_failure(1)
    assert "exit code 1" in plain
    assert "Manual inspection" in plain


def test_format_summary_updated():
    diff_result = {
        "updated": [{
            "name": "akkoma",
            "old_digest": "sha256:aaabbbcccddd111222333444555666",
            "new_digest": "sha256:777888999000aaabbbcccddd111222",
            "old_created": "t1",
            "new_created": "t2",
            "changelog_summary": None,
        }],
        "added": [],
        "removed": [],
        "unchanged": [],
    }
    plain, html = format_update_summary(diff_result, _CONFIG)
    assert "Akkoma" in plain
    assert "Updated (1)" in plain
    assert "sha256:aaabbbcccddd" in plain
    assert "sha256:777888999000" in plain


def test_format_summary_added():
    diff_result = {
        "updated": [],
        "added": [{"name": "writefreely", "digest": "sha256:abc", "changelog_summary": None}],
        "removed": [],
        "unchanged": [],
    }
    plain, html = format_update_summary(diff_result, _CONFIG)
    assert "Added (1)" in plain
    assert "writefreely (new)" in plain


def test_format_summary_uses_service_names():
    diff_result = {
        "updated": [{
            "name": "postgres",
            "old_digest": "sha256:aaa",
            "new_digest": "sha256:bbb",
            "old_created": "t1",
            "new_created": "t2",
            "changelog_summary": None,
        }],
        "added": [],
        "removed": [],
        "unchanged": [],
    }
    plain, html = format_update_summary(diff_result, _CONFIG)
    assert "PostgreSQL" in plain
    assert "postgres" not in plain.lower().split("postgresql")[0]


def test_format_summary_html_has_tags():
    diff_result = {
        "updated": [{
            "name": "akkoma",
            "old_digest": "sha256:aaa",
            "new_digest": "sha256:bbb",
            "old_created": "t1",
            "new_created": "t2",
            "changelog_summary": None,
        }],
        "added": [],
        "removed": [],
        "unchanged": [],
    }
    _, html = format_update_summary(diff_result, _CONFIG)
    assert "<strong>" in html
    assert "<li>" in html
