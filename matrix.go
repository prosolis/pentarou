package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	_ "modernc.org/sqlite"
)

// Notifier sends formatted messages to a Matrix room.
type Notifier interface {
	SendMessage(ctx context.Context, plain, html string) error
	Close()
}

// LegacyNotifier sends messages via raw HTTP PUT (no E2EE support).
type LegacyNotifier struct {
	cfg *Config
}

func (n *LegacyNotifier) SendMessage(_ context.Context, plain, html string) error {
	if !PostMessage(n.cfg.Matrix.Homeserver, n.cfg.Matrix.RoomID, n.cfg.Matrix.AccessToken, plain, html) {
		return fmt.Errorf("failed to send message via legacy notifier")
	}
	return nil
}

func (n *LegacyNotifier) Close() {}

// MatrixBot is a mautrix-go client with E2EE support.
type MatrixBot struct {
	client       *mautrix.Client
	cryptoHelper *cryptohelper.CryptoHelper
	roomID       id.RoomID
	syncCancel   context.CancelFunc
	syncDone     chan struct{}
}

// NewMatrixBot creates a mautrix client with E2EE support.
// It initializes the CryptoHelper for encrypted room communication
// and starts a background sync loop for crypto key exchange.
func NewMatrixBot(cfg *MatrixConfig) (*MatrixBot, error) {
	client, err := mautrix.NewClient(cfg.Homeserver, "", cfg.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("create mautrix client: %w", err)
	}

	// Derive UserID from the access token via /account/whoami.
	whoami, err := client.Whoami(context.Background())
	if err != nil {
		return nil, fmt.Errorf("whoami (is the homeserver reachable and the access token valid?): %w", err)
	}
	client.UserID = whoami.UserID
	client.DeviceID = id.DeviceID(cfg.DeviceID)
	log.Printf("INFO: Matrix bot authenticated as %s (device %s)", client.UserID, client.DeviceID)

	ch, err := cryptohelper.NewCryptoHelper(client, []byte(cfg.PickleKey), cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("init crypto helper: %w", err)
	}

	if err := ch.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("crypto helper init: %w", err)
	}

	client.Crypto = ch
	log.Printf("INFO: E2EE initialized (crypto store: %s)", cfg.DatabasePath)

	// Start background sync loop for crypto key exchange.
	syncCtx, syncCancel := context.WithCancel(context.Background())
	syncDone := make(chan struct{})
	go func() {
		defer close(syncDone)
		for {
			if err := client.SyncWithContext(syncCtx); err != nil {
				if syncCtx.Err() != nil {
					return
				}
				log.Printf("WARNING: sync error, retrying in 5s: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			return
		}
	}()

	return &MatrixBot{
		client:       client,
		cryptoHelper: ch,
		roomID:       id.RoomID(cfg.RoomID),
		syncCancel:   syncCancel,
		syncDone:     syncDone,
	}, nil
}

func (b *MatrixBot) SendMessage(ctx context.Context, plain, html string) error {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    plain,
	}
	if html != "" {
		content.Format = event.FormatHTML
		content.FormattedBody = html
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := b.client.SendMessageEvent(ctx, b.roomID, event.EventMessage, content)
		if err == nil {
			log.Printf("INFO: Message sent via mautrix")
			return nil
		}
		lastErr = err
		if attempt < maxRetries {
			wait := 1 << attempt
			log.Printf("WARNING: mautrix send failed (attempt %d/%d), retrying in %ds: %v",
				attempt, maxRetries, wait, err)
			time.Sleep(time.Duration(wait) * time.Second)
		}
	}
	return fmt.Errorf("send failed after %d attempts: %w", maxRetries, lastErr)
}

func (b *MatrixBot) Close() {
	b.syncCancel()
	<-b.syncDone
	b.cryptoHelper.Close()
	log.Printf("INFO: Matrix bot shut down")
}
