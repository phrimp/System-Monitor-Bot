package bot

import (
	"fmt"
	"system-monitor-bot/pkg/logger"

	"github.com/bwmarrin/discordgo"
)

func (sm *SystemMonitor) onReady(s *discordgo.Session, event *discordgo.Ready) {
	logger.Info("✅ Bot ready! Logged in as:", s.State.User.Username)

	// Set bot status
	s.UpdateGameStatus(0, "⚡ System Monitor Active")

	// Register slash commands
	sm.registerSlashCommands(s)
}

func (sm *SystemMonitor) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandName := i.ApplicationCommandData().Name
	userName := i.Member.User.Username

	logger.Info("📥 Received command:", commandName, "from user", userName)

	switch commandName {
	case "temp":
		sm.handleTemperatureCommand(s, i)
	case "ports":
		sm.handlePortsCommand(s, i)
	case "alerts":
		sm.handleAlertsCommand(s, i)
	case "status":
		sm.handleStatusCommand(s, i)
	default:
		logger.Warn("❓ Unknown command received:", commandName)
	}
}

func (sm *SystemMonitor) sendError(s *discordgo.Session, i *discordgo.InteractionCreate, title string, err error) {
	errorMsg := fmt.Sprintf("❌ **%s**\n```\n%v\n```", title, err)
	s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: errorMsg,
	})
}
