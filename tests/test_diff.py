from src.diff import compute_diff, has_changes


def _snapshot(containers):
    return {"timestamp": "2025-01-01T00:00:00Z", "containers": containers}


def test_no_changes():
    before = _snapshot({"app": {"image": "img:latest", "digest": "sha256:aaa", "created": "t1"}})
    after = _snapshot({"app": {"image": "img:latest", "digest": "sha256:aaa", "created": "t1"}})
    result = compute_diff(before, after)
    assert result["updated"] == []
    assert result["added"] == []
    assert result["removed"] == []
    assert result["unchanged"] == ["app"]
    assert not has_changes(result)


def test_updated_container():
    before = _snapshot({"app": {"image": "img:latest", "digest": "sha256:aaa", "created": "t1"}})
    after = _snapshot({"app": {"image": "img:latest", "digest": "sha256:bbb", "created": "t2"}})
    result = compute_diff(before, after)
    assert len(result["updated"]) == 1
    assert result["updated"][0]["name"] == "app"
    assert result["updated"][0]["old_digest"] == "sha256:aaa"
    assert result["updated"][0]["new_digest"] == "sha256:bbb"
    assert result["updated"][0]["changelog_summary"] is None
    assert has_changes(result)


def test_added_container():
    before = _snapshot({})
    after = _snapshot({"new-svc": {"image": "img:latest", "digest": "sha256:ccc", "created": "t1"}})
    result = compute_diff(before, after)
    assert len(result["added"]) == 1
    assert result["added"][0]["name"] == "new-svc"
    assert result["added"][0]["changelog_summary"] is None
    assert has_changes(result)


def test_removed_container():
    before = _snapshot({"old-svc": {"image": "img:latest", "digest": "sha256:ddd", "created": "t1"}})
    after = _snapshot({})
    result = compute_diff(before, after)
    assert len(result["removed"]) == 1
    assert result["removed"][0]["name"] == "old-svc"
    assert has_changes(result)


def test_mixed_changes():
    before = _snapshot({
        "stable": {"image": "a", "digest": "sha256:111", "created": "t1"},
        "updated": {"image": "b", "digest": "sha256:222", "created": "t1"},
        "removed": {"image": "c", "digest": "sha256:333", "created": "t1"},
    })
    after = _snapshot({
        "stable": {"image": "a", "digest": "sha256:111", "created": "t1"},
        "updated": {"image": "b", "digest": "sha256:444", "created": "t2"},
        "added": {"image": "d", "digest": "sha256:555", "created": "t2"},
    })
    result = compute_diff(before, after)
    assert len(result["updated"]) == 1
    assert len(result["added"]) == 1
    assert len(result["removed"]) == 1
    assert result["unchanged"] == ["stable"]


def test_empty_snapshots():
    result = compute_diff(_snapshot({}), _snapshot({}))
    assert not has_changes(result)
