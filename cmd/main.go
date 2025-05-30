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
	logger.Info("🚀 Starting System Monitor Bot...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("❌ Failed to load configuration:", err)
	}

	// Create and start bot
	systemBot, err := bot.New(cfg)
	if err != nil {
		logger.Fatal("❌ Failed to create bot:", err)
	}

	if err := systemBot.Start(); err != nil {
		logger.Fatal("❌ Failed to start bot:", err)
	}
	defer systemBot.Stop()

	logger.Info("🚀 System Monitor Bot is online!")

	// Wait for shutdown signal
	logger.Info("⏳ Waiting for shutdown signal...")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	logger.Info("🛑 Shutdown signal received, gracefully shutting down...")
	logger.Info("✅ System Monitor Bot shutdown complete")
}
