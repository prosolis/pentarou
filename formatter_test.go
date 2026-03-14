package main

import (
	"strings"
	"testing"
)

var testConfig = &Config{
	Notifications: NotificationsConfig{
		ServiceNames: map[string]string{
			"akkoma":   "Akkoma",
			"postgres": "PostgreSQL",
			"pixelfed": "PixelFed",
		},
	},
}

func TestParseBasicUpdate(t *testing.T) {
	msg := "Updating /mash-akkoma (sha256:aaabbb111222 to sha256:cccddd333444)"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].Name != "akkoma" {
		t.Errorf("expected name 'akkoma', got %q", updates[0].Name)
	}
	if updates[0].OldDigest != "sha256:aaabbb111222" {
		t.Errorf("unexpected old digest: %q", updates[0].OldDigest)
	}
	if updates[0].NewDigest != "sha256:cccddd333444" {
		t.Errorf("unexpected new digest: %q", updates[0].NewDigest)
	}
}

func TestParseWithContainerKeyword(t *testing.T) {
	msg := "Updating container /mash-pixelfed (sha256:aaa to sha256:bbb)"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].Name != "pixelfed" {
		t.Errorf("expected name 'pixelfed', got %q", updates[0].Name)
	}
}

func TestParseMultipleUpdates(t *testing.T) {
	msg := "Updating /mash-akkoma (sha256:aaa to sha256:bbb)\n" +
		"Updating /mash-postgres (sha256:ccc to sha256:ddd)\n" +
		"Updating /mash-pixelfed (sha256:eee to sha256:fff)"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(updates))
	}
	names := make(map[string]bool)
	for _, u := range updates {
		names[u.Name] = true
	}
	for _, name := range []string{"akkoma", "postgres", "pixelfed"} {
		if !names[name] {
			t.Errorf("expected %q in updates", name)
		}
	}
}

func TestParseNoUpdates(t *testing.T) {
	msg := "No containers need updating"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(updates))
	}
}

func TestParsePastTense(t *testing.T) {
	msg := "Updated /mash-akkoma (sha256:aaa to sha256:bbb)"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
}

func TestFormatReportWithUpdates(t *testing.T) {
	msg := "Updating /mash-akkoma (sha256:aaabbbcccddd111222333444 to sha256:777888999000aaabbbcccddd)\n" +
		"Updating /mash-postgres (sha256:111222333444555666777888 to sha256:aaabbbcccddd111222333444)"
	plain, html, ok := FormatUpdateReport(msg, testConfig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(plain, "Pentarou") {
		t.Error("expected 'Pentarou' in plain")
	}
	if !strings.Contains(plain, "Akkoma") {
		t.Error("expected 'Akkoma' in plain")
	}
	if !strings.Contains(plain, "PostgreSQL") {
		t.Error("expected 'PostgreSQL' in plain")
	}
	if !strings.Contains(plain, "sha256:aaabbbcccddd") {
		t.Error("expected short digest in plain")
	}
	if !strings.Contains(plain, "\u2192") {
		t.Error("expected arrow in plain")
	}
	if !strings.Contains(html, "<strong>") {
		t.Error("expected <strong> in html")
	}
}

func TestFormatReportNoUpdates(t *testing.T) {
	msg := "No containers need updating"
	_, _, ok := FormatUpdateReport(msg, testConfig)
	if ok {
		t.Error("expected ok=false for no updates")
	}
}

func TestFormatReportUsesServiceNames(t *testing.T) {
	msg := "Updating /mash-postgres (sha256:aaa to sha256:bbb)"
	plain, _, ok := FormatUpdateReport(msg, testConfig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(plain, "PostgreSQL") {
		t.Error("expected 'PostgreSQL' in plain")
	}
}

func TestFormatReportHTMLHasTags(t *testing.T) {
	msg := "Updating /mash-akkoma (sha256:aaa to sha256:bbb)"
	_, html, ok := FormatUpdateReport(msg, testConfig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(html, "<strong>") {
		t.Error("expected <strong> in html")
	}
	if !strings.Contains(html, "<li>") {
		t.Error("expected <li> in html")
	}
}

func TestFormatReportUnknownService(t *testing.T) {
	msg := "Updating /mash-somethingelse (sha256:aaa to sha256:bbb)"
	plain, _, ok := FormatUpdateReport(msg, testConfig)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(plain, "somethingelse") {
		t.Error("expected raw name 'somethingelse' in plain")
	}
}

func TestParseNoMashPrefix(t *testing.T) {
	msg := "Updating /mycontainer (sha256:aaa to sha256:bbb)"
	updates := ParseWatchtowerMessage(msg)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].Name != "mycontainer" {
		t.Errorf("expected name 'mycontainer', got %q", updates[0].Name)
	}
}
