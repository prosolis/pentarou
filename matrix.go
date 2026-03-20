package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	SendMessageToRoom(ctx context.Context, roomID, plain, html string) error
	Close()
}

// LegacyNotifier sends messages via raw HTTP PUT (no E2EE support).
type LegacyNotifier struct {
	cfg *Config
}

func (n *LegacyNotifier) SendMessage(ctx context.Context, plain, html string) error {
	return n.SendMessageToRoom(ctx, n.cfg.Matrix.RoomID, plain, html)
}

func (n *LegacyNotifier) SendMessageToRoom(ctx context.Context, roomID, plain, html string) error {
	return PostMessage(ctx, n.cfg.Matrix.Homeserver, roomID, n.cfg.Matrix.AccessToken, plain, html)
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
// It follows a two-tier auth strategy (matching gogobee):
//  1. Try stored device credentials from device.json → validate with /whoami
//  2. If no stored token or token invalid → login with username/password → save device.json
//  3. If an explicit access_token is set (config or env), skip the above and use it directly
//
// The cryptohelper is given LoginAs credentials so it can re-authenticate
// if the token expires. Cross-signing is bootstrapped on first run.
func NewMatrixBot(cfg *MatrixConfig) (*MatrixBot, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	devicePath := filepath.Join(cfg.DataDir, "device.json")
	var client *mautrix.Client

	if cfg.AccessToken != "" {
		// Explicit token from config or env var — use it directly.
		c, err := mautrix.NewClient(cfg.Homeserver, "", cfg.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("create mautrix client: %w", err)
		}
		whoami, err := c.Whoami(context.Background())
		if err != nil {
			return nil, fmt.Errorf("whoami (is the homeserver reachable and the access token valid?): %w", err)
		}
		c.UserID = whoami.UserID
		c.DeviceID = id.DeviceID(cfg.DeviceID)
		client = c
		log.Printf("INFO: Matrix bot authenticated as %s (device %s) via access token", client.UserID, client.DeviceID)
	} else {
		// Two-tier auth: try stored token, fallback to password login.
		device, err := loadDevice(devicePath)
		if err != nil {
			log.Printf("INFO: No existing device found, will login fresh")
		}

		if device != nil {
			valid, _ := IsTokenValid(cfg.Homeserver, device.AccessToken)
			if valid {
				log.Printf("INFO: Existing device credentials valid (device %s)", device.DeviceID)
				userID := id.UserID(device.UserID)
				c, err := mautrix.NewClient(cfg.Homeserver, userID, device.AccessToken)
				if err != nil {
					return nil, fmt.Errorf("create client with existing token: %w", err)
				}
				c.DeviceID = id.DeviceID(device.DeviceID)
				client = c
			} else {
				log.Printf("WARNING: Existing device credentials invalid, logging in again")
				device = nil
			}
		}

		if device == nil {
			// Fresh login with username/password.
			loginResp, err := LoginWithPassword(cfg.Homeserver, cfg.Username, cfg.Password, cfg.BotDisplayname)
			if err != nil {
				return nil, fmt.Errorf("login: %w", err)
			}

			userID := id.UserID(loginResp.UserID)
			c, err := mautrix.NewClient(cfg.Homeserver, userID, loginResp.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("create client: %w", err)
			}
			c.DeviceID = id.DeviceID(loginResp.DeviceID)
			client = c

			// Save device info for future reuse.
			info := &DeviceInfo{
				AccessToken: loginResp.AccessToken,
				DeviceID:    loginResp.DeviceID,
				UserID:      loginResp.UserID,
			}
			if err := saveDevice(devicePath, info); err != nil {
				log.Printf("WARNING: Failed to save device info: %v", err)
			} else {
				log.Printf("INFO: Device credentials saved to %s", devicePath)
			}

			log.Printf("INFO: Logged in as %s (device %s)", loginResp.UserID, loginResp.DeviceID)
		}
	}

	// Set up E2EE via cryptohelper with persistent SQLite crypto store.
	cryptoDBPath := filepath.Join(cfg.DataDir, cfg.DatabasePath)
	ch, err := cryptohelper.NewCryptoHelper(client, []byte(cfg.PickleKey), cryptoDBPath)
	if err != nil {
		return nil, fmt.Errorf("init crypto helper: %w", err)
	}

	// LoginAs enables the cryptohelper to re-login if the token expires,
	// and to bootstrap cross-signing on first run.
	if cfg.Username != "" && cfg.Password != "" {
		ch.LoginAs = &mautrix.ReqLogin{
			Type: mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: cfg.Username,
			},
			Password:                 cfg.Password,
			InitialDeviceDisplayName: cfg.BotDisplayname,
		}
	}

	if err := ch.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("crypto helper init: %w", err)
	}

	client.Crypto = ch

	// Bootstrap cross-signing if we have password credentials.
	if cfg.Username != "" && cfg.Password != "" {
		mach := ch.Machine()
		_, _, err := mach.GenerateAndUploadCrossSigningKeys(context.Background(), func(ui *mautrix.RespUserInteractive) interface{} {
			return map[string]interface{}{
				"type": mautrix.AuthTypePassword,
				"identifier": map[string]interface{}{
					"type": mautrix.IdentifierTypeUser,
					"user": cfg.Username,
				},
				"password": cfg.Password,
				"session":  ui.Session,
			}
		}, "")
		if err != nil {
			log.Printf("INFO: Cross-signing key upload skipped (may already exist): %v", err)
		} else {
			log.Printf("INFO: Cross-signing keys uploaded")
		}

		if err := mach.SignOwnDevice(context.Background(), mach.OwnIdentity()); err != nil {
			log.Printf("WARNING: Cross-signing: sign own device failed: %v", err)
		} else {
			log.Printf("INFO: Cross-signing: own device signed")
		}

		if err := mach.SignOwnMasterKey(context.Background()); err != nil {
			log.Printf("WARNING: Cross-signing: sign master key failed: %v", err)
		} else {
			log.Printf("INFO: Cross-signing: master key signed")
		}
	}

	log.Printf("INFO: E2EE initialized (crypto store: %s)", cryptoDBPath)

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
	return b.SendMessageToRoom(ctx, string(b.roomID), plain, html)
}

func (b *MatrixBot) SendMessageToRoom(ctx context.Context, roomID, plain, html string) error {
	targetRoom := id.RoomID(roomID)
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
		_, err := b.client.SendMessageEvent(ctx, targetRoom, event.EventMessage, content)
		if err == nil {
			log.Printf("INFO: Message sent via mautrix to %s", roomID)
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
