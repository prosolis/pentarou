package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type watchtowerPayload struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

func NewWebhookHandler(cfg *Config, notifier Notifier) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		const maxBodySize = 1 << 20 // 1 MB
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil || len(body) == 0 {
			log.Printf("WARNING: Webhook error 400: Empty or unreadable request body")
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}

		var payload watchtowerPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("WARNING: Webhook error 400: Invalid JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if payload.Message == "" {
			log.Printf("WARNING: Webhook error 400: Missing or empty 'message' field")
			http.Error(w, "Missing 'message' field", http.StatusBadRequest)
			return
		}

		log.Printf("INFO: Received webhook: title=%s level=%s", payload.Title, payload.Level)

		plain, html, hasUpdates := FormatUpdateReport(payload.Message, cfg)
		if !hasUpdates {
			if cfg.Notifications.SkipIfNoChanges {
				log.Printf("INFO: No updates in payload -- skipping notification")
			} else {
				log.Printf("INFO: No updates in payload (posting anyway per config)")
				noChangePlain := "🐧 **Pentarou — Update Report**\n\nAll containers are up to date."
				noChangeHTML := "<p>🐧 <strong>Pentarou — Update Report</strong></p><p>All containers are up to date.</p>"
				if err := notifier.SendMessage(r.Context(), noChangePlain, noChangeHTML); err != nil {
					log.Printf("ERROR: %v", err)
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if err := notifier.SendMessage(r.Context(), plain, html); err != nil {
			log.Printf("ERROR: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func RunServer(cfg *Config, notifier Notifier) *http.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Webhook.Host, cfg.Webhook.Port)
	handler := NewWebhookHandler(cfg, notifier)
	log.Printf("INFO: Pentarou listening on %s", addr)
	return &http.Server{Addr: addr, Handler: handler}
}
