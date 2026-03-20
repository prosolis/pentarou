package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseDirectPayload(t *testing.T) {
	body := []byte(`{
		"title": "Watchtower updates on myhost",
		"host": "myhost",
		"entries": [],
		"report": {
			"scanned": [],
			"updated": [
				{
					"id": "abc123",
					"name": "akkoma-akkoma-1",
					"imageName": "ghcr.io/akkoma-im/akkoma:latest",
					"currentImageId": "sha256:aaa111",
					"latestImageId": "sha256:bbb222",
					"state": "Updated"
				}
			],
			"failed": [],
			"skipped": [],
			"stale": [],
			"fresh": []
		}
	}`)

	payload, err := ParseWatchtowerPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Report.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(payload.Report.Updated))
	}
	u := payload.Report.Updated[0]
	if u.Name != "akkoma-akkoma-1" {
		t.Errorf("expected name 'akkoma-akkoma-1', got %q", u.Name)
	}
	if u.ImageName != "ghcr.io/akkoma-im/akkoma:latest" {
		t.Errorf("expected image 'ghcr.io/akkoma-im/akkoma:latest', got %q", u.ImageName)
	}
}

func TestParseShoutrrrWrapper(t *testing.T) {
	// Shoutrrr wraps the json.v1 output in {"message": "<json string>"}
	body := []byte(`{"message":"{\"title\":\"Watchtower\",\"host\":\"h\",\"report\":{\"updated\":[{\"name\":\"mash-traefik\",\"imageName\":\"traefik:latest\",\"state\":\"Updated\"}],\"scanned\":[],\"failed\":[],\"skipped\":[],\"stale\":[],\"fresh\":[]}}"}`)

	payload, err := ParseWatchtowerPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Report.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(payload.Report.Updated))
	}
	if payload.Report.Updated[0].Name != "mash-traefik" {
		t.Errorf("expected name 'mash-traefik', got %q", payload.Report.Updated[0].Name)
	}
}

func TestParseMultipleUpdated(t *testing.T) {
	body := []byte(`{
		"title": "Watchtower",
		"host": "h",
		"report": {
			"updated": [
				{"name": "akkoma-akkoma-1", "imageName": "ghcr.io/akkoma-im/akkoma:latest", "state": "Updated"},
				{"name": "mash-traefik", "imageName": "traefik:latest", "state": "Updated"},
				{"name": "akkoma-db-1", "imageName": "postgres:16", "state": "Updated"}
			],
			"scanned": [], "failed": [], "skipped": [], "stale": [], "fresh": []
		}
	}`)

	payload, err := ParseWatchtowerPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Report.Updated) != 3 {
		t.Fatalf("expected 3 updated, got %d", len(payload.Report.Updated))
	}
}

func TestParseNoReportData(t *testing.T) {
	body := []byte(`{"title": "test", "message": "just text, not json"}`)
	_, err := ParseWatchtowerPayload(body)
	if err == nil {
		t.Fatal("expected error for payload with no report data")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := ParseWatchtowerPayload([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseEmptyUpdated(t *testing.T) {
	body := []byte(`{
		"title": "Watchtower",
		"host": "h",
		"report": {
			"updated": [],
			"scanned": [{"name": "foo", "imageName": "foo:latest", "state": "Scanned"}],
			"failed": [], "skipped": [], "stale": [], "fresh": []
		}
	}`)

	payload, err := ParseWatchtowerPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Report.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(payload.Report.Updated))
	}
}

func TestFormatMappedWithRelease(t *testing.T) {
	entry := containerInfo{
		Name:      "akkoma-akkoma-1",
		ImageName: "ghcr.io/akkoma-im/akkoma:latest",
	}
	release := &GitHubRelease{
		TagName:  "v3.13.0",
		Body:     "## Changes\n- Fixed a bug",
		BodyHTML: "<h2>Changes</h2>\n<ul><li>Fixed a bug</li></ul>",
		HTMLURL:  "https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0",
	}

	plain, html := FormatContainerUpdate(entry, release, nil)

	for _, want := range []string{"akkoma-akkoma-1", "ghcr.io/akkoma-im/akkoma:latest", "v3.13.0", "Fixed a bug"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain missing %q", want)
		}
	}
	if !strings.Contains(plain, "🔗 https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0") {
		t.Error("plain missing release URL")
	}
	if !strings.Contains(html, "<h2>Changes</h2>") {
		t.Error("html missing rendered release body")
	}
	if !strings.Contains(html, `<a href="https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0"`) {
		t.Error("html missing release link")
	}
}

func TestFormatMappedGitHubError(t *testing.T) {
	entry := containerInfo{
		Name:      "mash-traefik",
		ImageName: "traefik:latest",
	}

	plain, html := FormatContainerUpdate(entry, nil, fmt.Errorf("rate limited"))

	if !strings.Contains(plain, "⚠️ Could not fetch release notes.") {
		t.Error("plain missing warning")
	}
	if !strings.Contains(html, "⚠️ Could not fetch release notes.") {
		t.Error("html missing warning")
	}
	if !strings.Contains(plain, "mash-traefik") {
		t.Error("plain missing container name")
	}
}

func TestFormatUnmapped(t *testing.T) {
	entry := containerInfo{
		Name:      "akkoma-db-1",
		ImageName: "postgres:16",
	}

	plain, html := FormatContainerUpdate(entry, nil, nil)

	if !strings.Contains(plain, "akkoma-db-1") {
		t.Error("plain missing container name")
	}
	if !strings.Contains(plain, "postgres:16") {
		t.Error("plain missing image name")
	}
	// Should NOT contain release notes or warning
	if strings.Contains(plain, "🏷️") {
		t.Error("unmapped container should not have tag")
	}
	if strings.Contains(plain, "⚠️") {
		t.Error("unmapped container should not have warning")
	}
	if !strings.Contains(html, "akkoma-db-1") {
		t.Error("html missing container name")
	}
}

func TestRepoMapLookup(t *testing.T) {
	tests := []struct {
		name   string
		mapped bool
		repo   string
	}{
		{"akkoma-akkoma-1", true, "akkoma-im/akkoma"},
		{"mash-traefik", true, "traefik/traefik"},
		{"mash-miniflux", true, "miniflux/miniflux"},
		{"akkoma-db-1", false, ""},
		{"unknown-container", false, ""},
	}

	for _, tt := range tests {
		repo, ok := RepoMap[tt.name]
		if ok != tt.mapped {
			t.Errorf("RepoMap[%q]: mapped=%v, want %v", tt.name, ok, tt.mapped)
		}
		if ok && repo != tt.repo {
			t.Errorf("RepoMap[%q]=%q, want %q", tt.name, repo, tt.repo)
		}
	}
}
