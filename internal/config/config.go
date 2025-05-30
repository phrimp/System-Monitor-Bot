package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Discord    DiscordConfig
	Monitor    MonitorConfig
	Thresholds ThresholdConfig
}

type DiscordConfig struct {
	Token   string
	GuildID string
}

type MonitorConfig struct {
	Interval      time.Duration
	AlertCooldown time.Duration
}

type ThresholdConfig struct {
	Critical float64
	Warning  float64
}

func Load() (*Config, error) {
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN environment variable is required")
	}

	return &Config{
		Discord: DiscordConfig{
			Token:   botToken,
			GuildID: os.Getenv("DISCORD_GUILD_ID"),
		},
		Monitor: MonitorConfig{
			Interval:      30 * time.Second,
			AlertCooldown: 5 * time.Minute,
		},
		Thresholds: ThresholdConfig{
			Critical: 80.0,
			Warning:  70.0,
		},
	}, nil
}
