import logging

log = logging.getLogger(__name__)


def compute_diff(before: dict, after: dict) -> dict:
    before_c = before.get("containers", {})
    after_c = after.get("containers", {})
    all_names = sorted(set(before_c) | set(after_c))

    updated = []
    added = []
    removed = []
    unchanged = []

    for name in all_names:
        in_before = name in before_c
        in_after = name in after_c

        if in_before and in_after:
            old = before_c[name]
            new = after_c[name]
            if old["digest"] != new["digest"]:
                updated.append({
                    "name": name,
                    "old_digest": old["digest"],
                    "new_digest": new["digest"],
                    "old_created": old.get("created"),
                    "new_created": new.get("created"),
                    "changelog_summary": None,
                })
            else:
                unchanged.append(name)
        elif in_after:
            added.append({
                "name": name,
                "digest": after_c[name]["digest"],
                "changelog_summary": None,
            })
        else:
            removed.append({
                "name": name,
                "digest": before_c[name]["digest"],
            })

    log.info(
        "Diff: %d updated, %d added, %d removed, %d unchanged",
        len(updated), len(added), len(removed), len(unchanged),
    )
    return {
        "updated": updated,
        "added": added,
        "removed": removed,
        "unchanged": unchanged,
    }


def has_changes(diff_result: dict) -> bool:
    return bool(diff_result["updated"] or diff_result["added"] or diff_result["removed"])
