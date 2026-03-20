package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Matrix        MatrixConfig        `yaml:"matrix"`
	Webhook       WebhookConfig       `yaml:"webhook"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

type MatrixConfig struct {
	Homeserver     string `yaml:"homeserver"`
	RoomID         string `yaml:"room_id"`
	AccessToken    string `yaml:"access_token"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	BotDisplayname string `yaml:"bot_displayname"`
	Encryption     bool   `yaml:"encryption"`
	DeviceID       string `yaml:"device_id"`
	DatabasePath   string `yaml:"database_path"`
	PickleKey      string `yaml:"pickle_key"`
	DataDir        string `yaml:"data_dir"`
}

type WebhookConfig struct {
	Host  string `yaml:"host"`
	Port  int    `yaml:"port"`
	Token string `yaml:"token"`
}

type NotificationsConfig struct {
	SkipIfNoChanges       bool   `yaml:"skip_if_no_changes"`
	WatchtowerUpdatesRoom string `yaml:"watchtower_updates_room"`
	GitHubToken           string `yaml:"github_token"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	cfg.Webhook.Host = "127.0.0.1"
	cfg.Webhook.Port = 8088
	cfg.Notifications.SkipIfNoChanges = true
	cfg.Matrix.Encryption = true
	cfg.Matrix.DeviceID = "PENTAROU"
	cfg.Matrix.DatabasePath = "pentarou-crypto.db"
	cfg.Matrix.PickleKey = "pentarou"
	cfg.Matrix.DataDir = "data"
	cfg.Matrix.BotDisplayname = "Pentarou"

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if token := os.Getenv("PENTAROU_MATRIX_TOKEN"); token != "" {
		cfg.Matrix.AccessToken = token
	}
	if ghToken := os.Getenv("PENTAROU_GITHUB_TOKEN"); ghToken != "" {
		cfg.Notifications.GitHubToken = ghToken
	}
	if whToken := os.Getenv("PENTAROU_WEBHOOK_TOKEN"); whToken != "" {
		cfg.Webhook.Token = whToken
	}
	if user := os.Getenv("PENTAROU_MATRIX_USER"); user != "" {
		cfg.Matrix.Username = user
	}
	if pass := os.Getenv("PENTAROU_MATRIX_PASSWORD"); pass != "" {
		cfg.Matrix.Password = pass
	}

	if cfg.Matrix.Homeserver == "" {
		return nil, fmt.Errorf("matrix.homeserver is required")
	}
	if cfg.Matrix.RoomID == "" {
		return nil, fmt.Errorf("matrix.room_id is required")
	}
	if cfg.Matrix.AccessToken == "" && (cfg.Matrix.Username == "" || cfg.Matrix.Password == "") {
		return nil, fmt.Errorf("matrix.access_token (or PENTAROU_MATRIX_TOKEN) is required, or provide matrix.username and matrix.password")
	}

	return &cfg, nil
}
