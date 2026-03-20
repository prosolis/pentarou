package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withTestGitHub(server *httptest.Server, fn func()) {
	old := githubBaseURL
	githubBaseURL = server.URL
	defer func() { githubBaseURL = old }()
	fn()
}

func TestFetchLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/akkoma-im/akkoma/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/vnd.github.full+json" {
			t.Error("missing full media type Accept header")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "v3.13.0",
			"body": "## Changes\n- Fixed a bug",
			"body_html": "<h2>Changes</h2>\n<ul><li>Fixed a bug</li></ul>",
			"html_url": "https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0"
		}`))
	}))
	defer server.Close()

	withTestGitHub(server, func() {
		release, err := FetchLatestRelease(context.Background(), "akkoma-im", "akkoma", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release.TagName != "v3.13.0" {
			t.Errorf("expected tag 'v3.13.0', got %q", release.TagName)
		}
		if release.Body != "## Changes\n- Fixed a bug" {
			t.Errorf("unexpected body: %q", release.Body)
		}
		if release.BodyHTML == "" {
			t.Error("expected non-empty body_html")
		}
		if release.HTMLURL != "https://github.com/akkoma-im/akkoma/releases/tag/v3.13.0" {
			t.Errorf("unexpected html_url: %q", release.HTMLURL)
		}
	})
}

func TestFetchLatestReleaseWithToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "v1.0.0",
			"body": "Release notes",
			"body_html": "<p>Release notes</p>",
			"html_url": "https://github.com/test/repo/releases/tag/v1.0.0"
		}`))
	}))
	defer server.Close()

	withTestGitHub(server, func() {
		_, err := FetchLatestRelease(context.Background(), "test", "repo", "ghp_testtoken123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedAuth != "Bearer ghp_testtoken123" {
			t.Errorf("expected auth header 'Bearer ghp_testtoken123', got %q", receivedAuth)
		}
	})
}

func TestFetchLatestReleaseNoToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.0.0","body":"","body_html":"","html_url":""}`))
	}))
	defer server.Close()

	withTestGitHub(server, func() {
		_, err := FetchLatestRelease(context.Background(), "test", "repo", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedAuth != "" {
			t.Errorf("expected no auth header, got %q", receivedAuth)
		}
	})
}

func TestFetchLatestReleaseNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	withTestGitHub(server, func() {
		_, err := FetchLatestRelease(context.Background(), "nonexistent", "repo", "")
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}

func TestFetchLatestReleaseMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	withTestGitHub(server, func() {
		_, err := FetchLatestRelease(context.Background(), "test", "repo", "")
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})
}
