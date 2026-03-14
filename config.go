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
	BotDisplayname string `yaml:"bot_displayname"`
}

type WebhookConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type NotificationsConfig struct {
	SkipIfNoChanges bool              `yaml:"skip_if_no_changes"`
	ServiceNames    map[string]string `yaml:"service_names"`
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

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if token := os.Getenv("PENTAROU_MATRIX_TOKEN"); token != "" {
		cfg.Matrix.AccessToken = token
	}

	if cfg.Matrix.Homeserver == "" {
		return nil, fmt.Errorf("matrix.homeserver is required")
	}
	if cfg.Matrix.RoomID == "" {
		return nil, fmt.Errorf("matrix.room_id is required")
	}
	if cfg.Matrix.AccessToken == "" {
		return nil, fmt.Errorf("matrix.access_token is required (set in config or PENTAROU_MATRIX_TOKEN env var)")
	}

	return &cfg, nil
}
