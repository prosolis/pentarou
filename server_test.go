package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
		SkipIfNoChanges: true,
	},
}

func TestWebhookReturns200OnValidPayload(t *testing.T) {
	handler := NewWebhookHandler(serverTestConfig)
	body := `{"title":"Watchtower updates","message":"No containers need updating","level":"info"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookReturns404OnWrongPath(t *testing.T) {
	handler := NewWebhookHandler(serverTestConfig)
	body := `{"message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/wrong", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWebhookReturns400OnMissingMessage(t *testing.T) {
	handler := NewWebhookHandler(serverTestConfig)
	body := `{"title":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookReturns400OnInvalidJSON(t *testing.T) {
	handler := NewWebhookHandler(serverTestConfig)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookReturns405OnGet(t *testing.T) {
	handler := NewWebhookHandler(serverTestConfig)
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
