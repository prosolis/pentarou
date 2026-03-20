package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
)

// watchtowerPayload is the Watchtower json.v1 notification format.
// When delivered via Shoutrrr's generic webhook, the json.v1 output may be
// wrapped in a {"message": "<json string>"} envelope — ParseWatchtowerPayload
// handles both cases automatically.
type watchtowerPayload struct {
	Title   string          `json:"title"`
	Host    string          `json:"host"`
	Message string          `json:"message"` // Shoutrrr wrapper field
	Report  containerReport `json:"report"`
}

type containerReport struct {
	Scanned []containerInfo `json:"scanned"`
	Updated []containerInfo `json:"updated"`
	Failed  []containerInfo `json:"failed"`
	Skipped []containerInfo `json:"skipped"`
	Stale   []containerInfo `json:"stale"`
	Fresh   []containerInfo `json:"fresh"`
}

type containerInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ImageName      string `json:"imageName"`
	CurrentImageID string `json:"currentImageId"`
	LatestImageID  string `json:"latestImageId"`
	State          string `json:"state"`
}

// RepoMap maps container names to GitHub owner/repo for release note lookups.
var RepoMap = map[string]string{
	"akkoma-akkoma-1":       "akkoma-im/akkoma",
	"lemmy-lemmy-1":         "LemmyNet/lemmy",
	"lemmy-lemmy-ui-1":      "LemmyNet/lemmy-ui",
	"lemmy-pictrs-1":        "asonix/pictrs",
	"mash-authentik-server": "goauthentik/authentik",
	"mash-miniflux":         "miniflux/miniflux",
	"mash-traefik":          "traefik/traefik",
	"mash-uptime-kuma":      "louislam/uptime-kuma",
	"mash-writefreely":      "writefreely/writefreely",
}

// ParseWatchtowerPayload parses a Watchtower json.v1 payload from raw bytes.
// It handles two delivery formats:
//   - Direct: the json.v1 JSON is the raw HTTP body (report field at top level)
//   - Shoutrrr wrapper: json.v1 is a JSON string inside {"message": "..."}
func ParseWatchtowerPayload(body []byte) (*watchtowerPayload, error) {
	var payload watchtowerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// If report is populated, this is a direct json.v1 payload.
	if hasReportData(&payload.Report) {
		return &payload, nil
	}

	// Otherwise, try Shoutrrr wrapper: the message field contains the json.v1 JSON string.
	if payload.Message != "" {
		var inner watchtowerPayload
		if err := json.Unmarshal([]byte(payload.Message), &inner); err != nil {
			return nil, fmt.Errorf("failed to parse json.v1 from message field: %w", err)
		}
		if hasReportData(&inner.Report) {
			return &inner, nil
		}
	}

	return nil, fmt.Errorf("payload has no report data (is WATCHTOWER_NOTIFICATION_TEMPLATE set to json.v1?)")
}

// hasReportData returns true if any report array is non-nil, indicating a valid json.v1 payload.
func hasReportData(r *containerReport) bool {
	return r.Updated != nil || r.Scanned != nil || r.Failed != nil ||
		r.Skipped != nil || r.Stale != nil || r.Fresh != nil
}

// FormatContainerUpdate builds a Matrix message for a single container update.
// All values interpolated into HTML are escaped to prevent injection.
func FormatContainerUpdate(entry containerInfo, release *GitHubRelease, fetchErr error) (plain, htmlOut string) {
	_, mapped := RepoMap[entry.Name]

	eName := html.EscapeString(entry.Name)
	eImage := html.EscapeString(entry.ImageName)

	if !mapped {
		// Unmapped container — minimal notification, no release notes.
		plain = fmt.Sprintf("🔔 Update available: %s\n📦 %s", entry.Name, entry.ImageName)
		htmlOut = fmt.Sprintf("<p>🔔 Update available: %s</p>\n<p>📦 %s</p>", eName, eImage)
		return
	}

	if release == nil || fetchErr != nil {
		// Mapped but GitHub API failed.
		plain = fmt.Sprintf("🔔 Update available: %s\n📦 %s\n⚠️ Could not fetch release notes.", entry.Name, entry.ImageName)
		htmlOut = fmt.Sprintf("<p>🔔 Update available: %s</p>\n<p>📦 %s</p>\n<p>⚠️ Could not fetch release notes.</p>", eName, eImage)
		if fetchErr != nil {
			log.Printf("WARNING: GitHub API error for %s: %v", entry.Name, fetchErr)
		}
		return
	}

	eTag := html.EscapeString(release.TagName)
	eURL := html.EscapeString(release.HTMLURL)

	// Mapped with release notes.
	// Plain text uses raw values (no HTML rendering). HTML uses escaped values
	// except for BodyHTML which is pre-rendered by GitHub's own sanitizer.
	plain = fmt.Sprintf("🔔 Update available: %s\n📦 %s\n🏷️ %s\n📝\n%s\n🔗 %s",
		entry.Name, entry.ImageName, release.TagName, release.Body, release.HTMLURL)

	htmlOut = fmt.Sprintf("<p>🔔 Update available: %s</p>\n<p>📦 %s</p>\n<p>🏷️ %s</p>\n📝\n%s\n<p>🔗 <a href=\"%s\">%s</a></p>",
		eName, eImage, eTag, release.BodyHTML, eURL, eURL)

	return
}
