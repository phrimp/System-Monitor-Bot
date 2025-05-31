package config

import (
	"fmt"
	"os"
	"system-monitor-bot/pkg/logger"
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
	logger.Info("Loading configuration from environment variables...")

	logger.Info("Reading DISCORD_BOT_TOKEN...")
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		logger.Error("DISCORD_BOT_TOKEN environment variable is not set")
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN environment variable is required")
	}
	logger.Info("Discord bot token loaded successfully (length:", len(botToken), "characters)")

	logger.Info("Reading DISCORD_GUILD_ID...")
	guildID := os.Getenv("DISCORD_GUILD_ID")
	if guildID != "" {
		logger.Info("Discord guild ID loaded:", guildID)
	} else {
		logger.Info("No guild ID specified - commands will be global")
	}

	config := &Config{
		Discord: DiscordConfig{
			Token:   botToken,
			GuildID: guildID,
		},
		Monitor: MonitorConfig{
			Interval:      30 * time.Second,
			AlertCooldown: 5 * time.Minute,
		},
		Thresholds: ThresholdConfig{
			Critical: 80.0,
			Warning:  70.0,
		},
	}

	logger.Info("Configuration created with defaults:")
	logger.Info("- Monitor interval:", config.Monitor.Interval)
	logger.Info("- Alert cooldown:", config.Monitor.AlertCooldown)
	logger.Info("- Critical threshold:", config.Thresholds.Critical, "°C")
	logger.Info("- Warning threshold:", config.Thresholds.Warning, "°C")

	return config, nil
}
