package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Update struct {
	Name      string
	OldDigest string
	NewDigest string
}

var updatePattern = regexp.MustCompile(
	`(?i)updat(?:ing|ed)\s+(?:container\s+)?` +
		`(/?[^\s]+)` +
		`\s+\((\S+)\s+to\s+(\S+)\)`,
)

func cleanContainerName(raw string) string {
	name := strings.TrimLeft(raw, "/")
	if strings.HasPrefix(name, "mash-") {
		name = name[5:]
	}
	return name
}

func shortDigest(digest string) string {
	if strings.HasPrefix(digest, "sha256:") && len(digest) > 19 {
		return "sha256:" + digest[7:19]
	}
	if len(digest) > 19 {
		return digest[:19]
	}
	return digest
}

func displayName(containerName string, serviceNames map[string]string) string {
	if name, ok := serviceNames[containerName]; ok {
		return name
	}
	return containerName
}

func ParseWatchtowerMessage(message string) []Update {
	matches := updatePattern.FindAllStringSubmatch(message, -1)
	updates := make([]Update, 0, len(matches))
	for _, m := range matches {
		updates = append(updates, Update{
			Name:      cleanContainerName(m[1]),
			OldDigest: m[2],
			NewDigest: m[3],
		})
	}
	return updates
}

func FormatUpdateReport(message string, cfg *Config) (plain, html string, ok bool) {
	updates := ParseWatchtowerMessage(message)
	if len(updates) == 0 {
		return "", "", false
	}

	serviceNames := cfg.Notifications.ServiceNames
	now := time.Now().UTC().Format("2006-01-02 15:04 UTC")

	var lines []string
	lines = append(lines, "\U0001f427 **Pentarou \u2014 Update Report**", "")

	for _, u := range updates {
		name := displayName(u.Name, serviceNames)
		old := shortDigest(u.OldDigest)
		new_ := shortDigest(u.NewDigest)
		lines = append(lines, fmt.Sprintf("- %s: `%s` \u2192 `%s`", name, old, new_))
	}

	lines = append(lines, "", now)

	plain = strings.Join(lines, "\n")
	html = markdownToHTML(plain)
	return plain, html, true
}

var (
	reCode = regexp.MustCompile("`([^`]+)`")
	reBold = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

func markdownToHTML(text string) string {
	html := reCode.ReplaceAllString(text, "<code>$1</code>")
	html = reBold.ReplaceAllString(html, "<strong>$1</strong>")

	lines := strings.Split(html, "\n")
	var result []string
	inList := false

	for _, line := range lines {
		if strings.HasPrefix(line, "- ") {
			if !inList {
				result = append(result, "<ul>")
				inList = true
			}
			result = append(result, "<li>"+line[2:]+"</li>")
		} else {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			if strings.TrimSpace(line) != "" {
				result = append(result, "<p>"+line+"</p>")
			}
		}
	}
	if inList {
		result = append(result, "</ul>")
	}

	return strings.Join(result, "\n")
}
