package main

import (
	"os"
	"os/signal"
	"syscall"
	"system-monitor-bot/internal/bot"
	"system-monitor-bot/internal/config"
	"system-monitor-bot/pkg/logger"
)

func main() {
	// Initialize logger
	logger.Init()
	logger.Info("Starting System Monitor Bot...")
	logger.Info("Go runtime initialized")

	// Load configuration
	logger.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration:", err)
	}
	logger.Info("Configuration loaded successfully")
	logger.Info("Discord Guild ID:", cfg.Discord.GuildID)
	logger.Info("Monitor interval:", cfg.Monitor.Interval)
	logger.Info("Alert cooldown:", cfg.Monitor.AlertCooldown)
	logger.Info("Temperature thresholds - Warning:", cfg.Thresholds.Warning, "Critical:", cfg.Thresholds.Critical)

	// Create and start bot
	logger.Info("Creating bot instance...")
	systemBot, err := bot.New(cfg)
	if err != nil {
		logger.Fatal("Failed to create bot:", err)
	}
	logger.Info("Bot instance created successfully")

	logger.Info("Starting bot...")
	if err := systemBot.Start(); err != nil {
		logger.Fatal("Failed to start bot:", err)
	}
	defer func() {
		logger.Info("Stopping bot...")
		systemBot.Stop()
		logger.Info("Bot stopped")
	}()

	logger.Info("System Monitor Bot is online!")

	// Wait for shutdown signal
	logger.Info("Waiting for shutdown signal...")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	sig := <-stop

	logger.Info("Shutdown signal received:", sig.String())
	logger.Info("Gracefully shutting down...")
	logger.Info("System Monitor Bot shutdown complete")
}
