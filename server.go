package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	maxConcurrentFetches = 4
	matrixSendDelay      = 500 * time.Millisecond
)

// containerUpdate pairs a container with the kind of update event it represents.
type containerUpdate struct {
	info containerInfo
	kind updateKind
}

// collectUpdates returns the containers that have an available update, merging
// Watchtower's "updated" bucket (populated in auto-update mode) with its "stale"
// bucket (populated in monitor-only mode). Each entry records which bucket it
// came from so the notification can distinguish an applied update from an
// available one. Entries are deduplicated by container ID, falling back to name
// when ID is empty; the "updated" bucket is processed first so an applied update
// wins over a stale duplicate.
func collectUpdates(r *containerReport) []containerUpdate {
	out := make([]containerUpdate, 0, len(r.Updated)+len(r.Stale))
	seen := make(map[string]struct{}, len(r.Updated)+len(r.Stale))
	groups := []struct {
		containers []containerInfo
		kind       updateKind
	}{
		{r.Updated, updateApplied},
		{r.Stale, updateAvailable},
	}
	for _, group := range groups {
		for _, c := range group.containers {
			key := c.ID
			if key == "" {
				key = c.Name
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, containerUpdate{info: c, kind: group.kind})
		}
	}
	return out
}

func NewWebhookHandler(cfg *Config, notifier Notifier) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if cfg.Webhook.Token != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+cfg.Webhook.Token {
				log.Printf("WARNING: Webhook error 401: invalid or missing Authorization header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		const maxBodySize = 1 << 20 // 1 MB
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil || len(body) == 0 {
			log.Printf("WARNING: Webhook error 400: Empty or unreadable request body")
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}

		payload, err := ParseWatchtowerPayload(body)
		if err != nil {
			log.Printf("WARNING: Webhook error 400: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Collect containers with an available update. In monitor-only mode
		// Watchtower never actually updates anything, so it reports available
		// updates in the "stale" bucket and leaves "updated" empty. In
		// auto-update mode the reverse is true. Merge both (deduped) so Pentarou
		// announces updates regardless of which mode Watchtower runs in.
		updated := collectUpdates(&payload.Report)

		log.Printf("INFO: Received webhook: title=%q host=%q updates=%d (updated=%d stale=%d) failed=%d",
			payload.Title, payload.Host, len(updated),
			len(payload.Report.Updated), len(payload.Report.Stale), len(payload.Report.Failed))

		if len(updated) == 0 {
			if cfg.Notifications.SkipIfNoChanges {
				log.Printf("INFO: No updates in payload -- skipping notification")
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		updatesRoom := cfg.Notifications.WatchtowerUpdatesRoom
		if updatesRoom == "" {
			updatesRoom = cfg.Matrix.RoomID
		}

		// Fetch GitHub releases concurrently for mapped containers.
		type fetchResult struct {
			release *GitHubRelease
			err     error
		}
		results := make([]fetchResult, len(updated))
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxConcurrentFetches)

		ctx := r.Context()

		for i, entry := range updated {
			repoPath, mapped := RepoMap[entry.info.Name]
			if !mapped {
				continue
			}
			wg.Add(1)
			go func(i int, repoPath string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					results[i] = fetchResult{err: ctx.Err()}
					return
				}
				defer func() { <-sem }()

				parts := strings.SplitN(repoPath, "/", 2)
				if len(parts) != 2 {
					results[i] = fetchResult{err: fmt.Errorf("invalid repo path in map: %q", repoPath)}
					return
				}
				rel, err := FetchLatestRelease(ctx, parts[0], parts[1], cfg.Notifications.GitHubToken)
				results[i] = fetchResult{release: rel, err: err}
			}(i, repoPath)
		}
		wg.Wait()

		// Send one Matrix message per container, with delay between sends.
		for i, entry := range updated {
			if ctx.Err() != nil {
				log.Printf("WARNING: context cancelled, skipping remaining %d notifications", len(updated)-i)
				break
			}
			plain, html := FormatContainerUpdate(entry.info, entry.kind, results[i].release, results[i].err)
			if err := notifier.SendMessageToRoom(ctx, updatesRoom, plain, html); err != nil {
				log.Printf("ERROR: failed to send update for %s: %v", entry.info.Name, err)
			}
			if i < len(updated)-1 {
				select {
				case <-time.After(matrixSendDelay):
				case <-ctx.Done():
				}
			}
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
