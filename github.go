package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxGitHubResponseSize = 5 << 20 // 5 MB
	githubHTTPTimeout     = 15 * time.Second
	defaultGitHubBaseURL  = "https://api.github.com"
)

var githubHTTPClient = &http.Client{Timeout: githubHTTPTimeout}

// githubBaseURL is the base URL for GitHub API requests.
// Override in tests to point at an httptest server.
var githubBaseURL = defaultGitHubBaseURL

// GitHubRelease holds the fields we care about from the GitHub releases API.
type GitHubRelease struct {
	TagName  string `json:"tag_name"`
	Body     string `json:"body"`      // raw Markdown
	BodyHTML string `json:"body_html"` // pre-rendered HTML (requires full media type)
	HTMLURL  string `json:"html_url"`
}

// FetchLatestRelease calls the GitHub releases API for the given owner/repo.
// It uses the "full" media type so the response includes body, body_text, and
// body_html — no local Markdown rendering needed.
// The token parameter is optional; if non-empty it is sent as a Bearer token
// for higher rate limits.
func FetchLatestRelease(ctx context.Context, owner, repo, token string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubBaseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.full+json")
	req.Header.Set("User-Agent", "Pentarou/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var release GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxGitHubResponseSize)).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub release: %w", err)
	}

	return &release, nil
}
