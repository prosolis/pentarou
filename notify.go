package main

import (
	"bytes"
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

type matrixMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format,omitempty"`
	FormattedBody string `json:"formatted_body,omitempty"`
}

func PostMessage(homeserver, roomID, accessToken, plainBody, htmlBody string) bool {
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

	client := &http.Client{Timeout: httpTimeout}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
		if err != nil {
			log.Printf("ERROR: failed to create request: %v", err)
			return false
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				wait := 1 << attempt // 2, 4
				log.Printf("WARNING: Matrix post failed (attempt %d/%d), retrying in %ds: %v",
					attempt, maxRetries, wait, err)
				time.Sleep(time.Duration(wait) * time.Second)
				continue
			}
			log.Printf("ERROR: Matrix post failed after %d attempts: %v", maxRetries, err)
			return false
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("INFO: Message sent (status %d)", resp.StatusCode)
			return true
		}

		if attempt < maxRetries {
			wait := 1 << attempt
			log.Printf("WARNING: Matrix post returned %d (attempt %d/%d), retrying in %ds",
				resp.StatusCode, attempt, maxRetries, wait)
			time.Sleep(time.Duration(wait) * time.Second)
		} else {
			log.Printf("ERROR: Matrix post failed after %d attempts (status %d)", maxRetries, resp.StatusCode)
		}
	}
	return false
}

