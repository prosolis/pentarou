package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sentMessage struct {
	roomID string
	plain  string
	html   string
}

type mockNotifier struct {
	messages []sentMessage
	sendErr  error
}

func (m *mockNotifier) SendMessage(ctx context.Context, plain, html string) error {
	return m.SendMessageToRoom(ctx, "", plain, html)
}

func (m *mockNotifier) SendMessageToRoom(_ context.Context, roomID, plain, html string) error {
	m.messages = append(m.messages, sentMessage{roomID: roomID, plain: plain, html: html})
	return m.sendErr
}

func (m *mockNotifier) Close() {}

var serverTestConfig = &Config{
	Matrix: MatrixConfig{
		Homeserver:  "http://127.0.0.1:1",
		RoomID:      "!test:example.com",
		AccessToken: "testtoken",
	},
	Webhook: WebhookConfig{
		Host: "127.0.0.1",
		Port: 0,
	},
	Notifications: NotificationsConfig{
		SkipIfNoChanges:       true,
		WatchtowerUpdatesRoom: "!updates:example.com",
	},
}

func validPayload(containers ...string) string {
	var entries []string
	for _, c := range containers {
		entries = append(entries, fmt.Sprintf(
			`{"name":%q,"imageName":"img/%s:latest","currentImageId":"sha256:aaa","latestImageId":"sha256:bbb","state":"Updated"}`, c, c))
	}
	return fmt.Sprintf(`{
		"title":"Watchtower updates","host":"h",
		"report":{
			"updated":[%s],
			"scanned":[],"failed":[],"skipped":[],"stale":[],"fresh":[]
		}
	}`, strings.Join(entries, ","))
}

func TestWebhookReturns200OnValidPayload(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if mock.messages[0].roomID != "!updates:example.com" {
		t.Errorf("expected room '!updates:example.com', got %q", mock.messages[0].roomID)
	}
}

func TestWebhookFallsBackToDefaultRoom(t *testing.T) {
	cfg := *serverTestConfig
	cfg.Notifications.WatchtowerUpdatesRoom = ""
	mock := &mockNotifier{}
	handler := NewWebhookHandler(&cfg, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if mock.messages[0].roomID != "!test:example.com" {
		t.Errorf("expected fallback room '!test:example.com', got %q", mock.messages[0].roomID)
	}
}

func TestWebhookReturns404OnWrongPath(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	req := httptest.NewRequest(http.MethodPost, "/wrong", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWebhookReturns400OnInvalidJSON(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookReturns405OnGet(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestWebhookSkipsWhenNoUpdates(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := `{
		"title":"Watchtower","host":"h",
		"report":{
			"updated":[],"scanned":[{"name":"foo","imageName":"foo:latest","state":"Scanned"}],
			"failed":[],"skipped":[],"stale":[],"fresh":[]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(mock.messages))
	}
}

func TestWebhookSendsOneMessagePerContainer(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := validPayload("akkoma-db-1", "mash-valkey", "lemmy-postgres-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.messages) != 3 {
		t.Fatalf("expected 3 messages (one per container), got %d", len(mock.messages))
	}
	// validPayload puts containers in the "updated" bucket (auto-update mode),
	// so each should be announced as an applied update.
	for _, msg := range mock.messages {
		if !strings.Contains(msg.plain, "✅ Updated:") {
			t.Errorf("message missing applied-update header: %q", msg.plain)
		}
	}
}

// stalePayload mimics Watchtower running in monitor-only mode: available
// updates land in the "stale" bucket and "updated" is always empty.
func stalePayload(containers ...string) string {
	var entries []string
	for _, c := range containers {
		entries = append(entries, fmt.Sprintf(
			`{"name":%q,"imageName":"img/%s:latest","currentImageId":"sha256:aaa","latestImageId":"sha256:bbb","state":"Stale"}`, c, c))
	}
	return fmt.Sprintf(`{
		"title":"Watchtower updates","host":"h",
		"report":{
			"updated":[],
			"scanned":[],"failed":[],"skipped":[],"stale":[%s],"fresh":[]
		}
	}`, strings.Join(entries, ","))
}

func TestWebhookAnnouncesStaleContainersMonitorOnly(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := stalePayload("mash-valkey", "lemmy-postgres-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.messages) != 2 {
		t.Fatalf("expected 2 messages for stale containers, got %d", len(mock.messages))
	}
	// Monitor-only mode: containers are stale, not yet updated.
	for _, msg := range mock.messages {
		if !strings.Contains(msg.plain, "🔔 Update available:") {
			t.Errorf("stale message should report '🔔 Update available:', got %q", msg.plain)
		}
	}
}

func TestCollectUpdatesDedupesUpdatedAndStale(t *testing.T) {
	r := &containerReport{
		Updated: []containerInfo{{ID: "1", Name: "a"}},
		Stale:   []containerInfo{{ID: "1", Name: "a"}, {ID: "2", Name: "b"}},
	}
	got := collectUpdates(r)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped updates, got %d: %+v", len(got), got)
	}
	// Container "a" appears in both buckets; the "updated" bucket wins, so it
	// is reported as an applied update rather than merely available.
	if got[0].info.Name != "a" || got[0].kind != updateApplied {
		t.Errorf("expected container 'a' to be updateApplied, got %+v", got[0])
	}
	if got[1].info.Name != "b" || got[1].kind != updateAvailable {
		t.Errorf("expected container 'b' to be updateAvailable, got %+v", got[1])
	}
}

func TestWebhookHandlesShoutrrrWrapper(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	// json.v1 payload wrapped in Shoutrrr's {"message": "..."} envelope
	body := `{"message":"{\"title\":\"Watchtower\",\"host\":\"h\",\"report\":{\"updated\":[{\"name\":\"akkoma-db-1\",\"imageName\":\"postgres:16\",\"state\":\"Updated\"}],\"scanned\":[],\"failed\":[],\"skipped\":[],\"stale\":[],\"fresh\":[]}}"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if !strings.Contains(mock.messages[0].plain, "akkoma-db-1") {
		t.Error("message missing container name")
	}
}

func TestWebhookReturns400OnMissingReport(t *testing.T) {
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := `{"title":"test","message":"just plain text, not json.v1"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookRejectsWithoutToken(t *testing.T) {
	cfg := *serverTestConfig
	cfg.Webhook.Token = "secret123"
	mock := &mockNotifier{}
	handler := NewWebhookHandler(&cfg, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookRejectsWrongToken(t *testing.T) {
	cfg := *serverTestConfig
	cfg.Webhook.Token = "secret123"
	mock := &mockNotifier{}
	handler := NewWebhookHandler(&cfg, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookAcceptsCorrectToken(t *testing.T) {
	cfg := *serverTestConfig
	cfg.Webhook.Token = "secret123"
	mock := &mockNotifier{}
	handler := NewWebhookHandler(&cfg, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookNoAuthRequiredWhenTokenEmpty(t *testing.T) {
	// serverTestConfig has no token set — auth should be skipped
	mock := &mockNotifier{}
	handler := NewWebhookHandler(serverTestConfig, mock)
	body := validPayload("akkoma-db-1")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
