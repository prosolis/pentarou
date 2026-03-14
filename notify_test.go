package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostMessageSendsCorrectly(t *testing.T) {
	var receivedAuth string
	var receivedBody matrixMessage
	var receivedMethod string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"event_id":"$test123"}`))
	}))
	defer server.Close()

	result := PostMessage(server.URL, "!test:example.com", "syt_testtoken", "Hello", "<p>Hello</p>")

	if !result {
		t.Fatal("expected result=true")
	}
	if receivedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	if !strings.Contains(receivedPath, "/_matrix/client/v3/rooms/") {
		t.Error("expected Matrix room path")
	}
	if receivedAuth != "Bearer syt_testtoken" {
		t.Errorf("unexpected auth header: %q", receivedAuth)
	}
	if receivedBody.MsgType != "m.text" {
		t.Errorf("expected msgtype m.text, got %q", receivedBody.MsgType)
	}
	if receivedBody.Body != "Hello" {
		t.Errorf("expected body 'Hello', got %q", receivedBody.Body)
	}
	if receivedBody.FormattedBody != "<p>Hello</p>" {
		t.Errorf("expected formatted_body '<p>Hello</p>', got %q", receivedBody.FormattedBody)
	}
}

func TestPostMessagePlainOnly(t *testing.T) {
	var receivedBody matrixMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Write([]byte(`{"event_id":"$test123"}`))
	}))
	defer server.Close()

	result := PostMessage(server.URL, "!test:example.com", "token", "Plain message", "")

	if !result {
		t.Fatal("expected result=true")
	}
	if receivedBody.FormattedBody != "" {
		t.Error("expected no formatted_body for plain-only message")
	}
	if receivedBody.Format != "" {
		t.Error("expected no format for plain-only message")
	}
}

func TestPostMessageRetriesOnFailure(t *testing.T) {
	result := PostMessage("http://127.0.0.1:1", "!test:example.com", "token", "Should fail", "")
	if result {
		t.Error("expected result=false for unreachable server")
	}
}
