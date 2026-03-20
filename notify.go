package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const (
	maxRetries  = 3
	httpTimeout = 30 * time.Second
)

var matrixHTTPClient = &http.Client{Timeout: httpTimeout}

type matrixMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format,omitempty"`
	FormattedBody string `json:"formatted_body,omitempty"`
}

func PostMessage(ctx context.Context, homeserver, roomID, accessToken, plainBody, htmlBody string) error {
	txnID := uuid.New().String()
	encodedRoom := url.PathEscape(roomID)
	endpoint := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		homeserver, encodedRoom, txnID)

	msg := matrixMessage{
		MsgType: "m.text",
		Body:    plainBody,
	}
	if htmlBody != "" {
		msg.Format = "org.matrix.custom.html"
		msg.FormattedBody = htmlBody
	}

	data, _ := json.Marshal(msg)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}

		resp, err := matrixHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				wait := 1 << attempt // 2, 4
				log.Printf("WARNING: Matrix post failed (attempt %d/%d), retrying in %ds: %v",
					attempt, maxRetries, wait, err)
				select {
				case <-time.After(time.Duration(wait) * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}
			return fmt.Errorf("Matrix post failed after %d attempts: %w", maxRetries, lastErr)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("INFO: Message sent (status %d)", resp.StatusCode)
			return nil
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		if attempt < maxRetries {
			wait := 1 << attempt
			log.Printf("WARNING: Matrix post returned %d (attempt %d/%d), retrying in %ds",
				resp.StatusCode, attempt, maxRetries, wait)
			select {
			case <-time.After(time.Duration(wait) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return fmt.Errorf("Matrix post failed after %d attempts: %w", maxRetries, lastErr)
}
